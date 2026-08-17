-- Schema for the example module. compose.yaml mounts this into the postgres
-- container's entrypoint, which runs it the first time the data volume is created.
-- Re-run it by hand, or `docker compose down -v`, after you change it.

CREATE TABLE IF NOT EXISTS example_notes (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title      VARCHAR(200) NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
