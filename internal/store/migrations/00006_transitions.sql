-- +goose Up
CREATE TABLE IF NOT EXISTS transitions (
    previous TEXT NOT NULL REFERENCES commands(command) ON DELETE CASCADE ON UPDATE CASCADE,
    command TEXT NOT NULL REFERENCES commands(command) ON DELETE CASCADE ON UPDATE CASCADE,
    use_count INTEGER NOT NULL DEFAULT 1,
    last_used_at INTEGER NOT NULL,
    PRIMARY KEY (previous, command)
);

CREATE INDEX IF NOT EXISTS transitions_command_idx ON transitions(command);
CREATE INDEX IF NOT EXISTS transitions_last_used_at_idx ON transitions(last_used_at);
