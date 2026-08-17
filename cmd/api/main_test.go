package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// validConfig is the smallest config that loads, rendered with the port under test.
func validConfig(port int) string {
	return fmt.Sprintf(`
app:
  name: template
  version: 1.0.0
  env: development
server:
  mode: test
  port: %d
  timeout: 5s
  trusted_proxies: []
logger:
  level: INFO
`, port)
}

// writeConfig puts a config where Load looks for it, from an empty working directory.
func writeConfig(t *testing.T, contents string) {
	t.Helper()

	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.MkdirAll("configs", 0o750); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	if err := os.WriteFile(filepath.Join("configs", "config.yaml"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// freePort returns a port and the listener holding it. run binds the wildcard address,
// so this must too, or the second bind quietly succeeds and run blocks on its signal wait.
func freePort(t *testing.T) (int, net.Listener) {
	t.Helper()

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address is %T, want *net.TCPAddr", listener.Addr())
	}
	return addr.Port, listener
}

func TestRunReportsMissingConfig(t *testing.T) {
	t.Chdir(t.TempDir())

	err := run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "config.yaml") {
		t.Fatalf("error = %v, want it to name the missing config", err)
	}
}

func TestRunReportsUnbuildableLogger(t *testing.T) {
	port, listener := freePort(t)
	_ = listener.Close()
	writeConfig(t, validConfig(port))

	// A file where the logger wants its directory stops the container being built.
	if err := os.WriteFile("logs", []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write logs: %v", err)
	}

	if err := run(context.Background()); err == nil || !strings.Contains(err.Error(), "logger") {
		t.Fatalf("error = %v, want a logger failure", err)
	}
}

// NewRouter's error branch is unreachable from here: config validation enforces
// `ip|cidr` on trusted_proxies, so anything that survives Load also satisfies gin.
// Reaching it needs server.NewRouter directly, which internal/api/server covers.

// A port already in use has to be reported, not swallowed into a silent exit.
func TestRunReportsUnavailablePort(t *testing.T) {
	port, listener := freePort(t)
	defer func() { _ = listener.Close() }()
	writeConfig(t, validConfig(port))

	err := run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "listen") {
		t.Fatalf("error = %v, want the bind failure", err)
	}
}

// The whole lifecycle: listen, serve real traffic, then shut down when ctx is cancelled.
func TestRunServesUntilContextIsCancelled(t *testing.T) {
	port, listener := freePort(t)
	_ = listener.Close()
	writeConfig(t, validConfig(port))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()

	// Poll until the listener is up rather than sleeping a guessed interval.
	var resp *http.Response
	for range 100 {
		var err error
		resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if resp == nil {
		t.Fatal("server never became reachable")
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "Healthy") {
		t.Errorf("GET /health = %d %q", resp.StatusCode, body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run returned %v, want a clean shutdown", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("run did not return after the context was cancelled")
	}
}
