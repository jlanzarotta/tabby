-- +goose NO TRANSACTION
-- +goose Up
-- Clears commands that were deleted before secure_delete was turned on.
VACUUM;
PRAGMA wal_checkpoint(TRUNCATE);
