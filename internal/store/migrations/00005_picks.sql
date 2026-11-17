-- +goose Up
CREATE TABLE IF NOT EXISTS picks (
    query TEXT NOT NULL,
    command TEXT NOT NULL REFERENCES commands(command) ON DELETE CASCADE ON UPDATE CASCADE,
    use_count INTEGER NOT NULL DEFAULT 1,
    last_used_at INTEGER NOT NULL,
    PRIMARY KEY (query, command)
);

CREATE INDEX IF NOT EXISTS picks_command_idx ON picks(command);
CREATE INDEX IF NOT EXISTS picks_last_used_at_idx ON picks(last_used_at);
