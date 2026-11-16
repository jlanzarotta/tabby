-- +goose Up
ALTER TABLE commands ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS command_dirs (
    command TEXT NOT NULL REFERENCES commands(command) ON DELETE CASCADE ON UPDATE CASCADE,
    dir TEXT NOT NULL,
    root TEXT NOT NULL DEFAULT '',
    use_count INTEGER NOT NULL DEFAULT 1,
    last_used_at INTEGER NOT NULL,
    PRIMARY KEY (command, dir)
);

CREATE INDEX IF NOT EXISTS command_dirs_dir_idx ON command_dirs(dir);
CREATE INDEX IF NOT EXISTS command_dirs_root_idx ON command_dirs(root) WHERE root <> '';
CREATE INDEX IF NOT EXISTS command_dirs_last_used_at_idx ON command_dirs(last_used_at);
