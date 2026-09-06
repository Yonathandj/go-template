package database

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestSQLServerDSN(t *testing.T) {
	cases := map[string]string{
		"": "sqlserver://user:s4cr4t@localhost:2222?database=go",
		"encrypt=true&connection+timeout=40": "sqlserver://user:s4cr4t@localhost:2222?database=go" +
			"&encrypt=true&connection+timeout=40",
	}
	for opts, want := range cases {
		if got := sqlServerDSN("localhost", 2222, "user", "s4cr4t", "go", opts); got != want {
			t.Errorf("sqlServerDSN(opts=%q) = %q, want %q", opts, got, want)
		}
	}
}

func TestSQLServerDSNEscapesPassword(t *testing.T) {
	got := sqlServerDSN("localhost", 2222, "user", "p@ss:w/rd?&", "go", "")
	want := "sqlserver://user:p%40ss%3Aw%2Frd%3F%26@localhost:2222?database=go"
	if got != want {
		t.Errorf("sqlServerDSN = %q, want %q", got, want)
	}
}

func TestNewSQLServer(t *testing.T) {
	db, mock := mockDB(t)
	mock.ExpectPing()

	orig := gormOpen
	gormOpen = func(gorm.Dialector, ...gorm.Option) (*gorm.DB, error) { return db, nil }
	t.Cleanup(func() { gormOpen = orig })

	got, err := NewSQLServer(
		"localhost", 2222, "user", "password", "database", "encrypt=true", PoolConfig{MaxIdleConns: 2})
	if err != nil {
		t.Fatalf("NewSQLServer: %v", err)
	}
	if got != db {
		t.Error("expected the opened DB back")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}

	mock.ExpectPing().WillReturnError(errors.New("boom"))
	if _, err := NewSQLServer(
		"localhost", 2222, "user", "password", "database", "encrypt=strict", PoolConfig{}); err == nil {
		t.Error("expected ping failure to abort")
	}

	gormOpen = func(gorm.Dialector, ...gorm.Option) (*gorm.DB, error) { return brokenDB(), nil }
	if _, err := NewSQLServer(
		"localhost", 2222, "user", "password", "database", "encrypt=true", PoolConfig{}); err == nil {
		t.Error("expected configurePool failure")
	}

	gormOpen = func(gorm.Dialector, ...gorm.Option) (*gorm.DB, error) { return nil, errors.New("boom") }
	if _, err := NewSQLServer(
		"localhost", 2222, "user", "password", "database", "encrypt=true", PoolConfig{}); err == nil {
		t.Error("expected open failure")
	}
}

func TestNewSQLServerUnreachable(t *testing.T) {
	if _, err := NewSQLServer(
		"127.0.0.2", 2, "user", "password", "database", "dial+timeout=2", PoolConfig{}); err == nil {
		t.Fatal("expected connection error")
	}
}
