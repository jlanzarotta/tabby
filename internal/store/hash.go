package store

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"
)

// retentionMonths is how long commands, directory usage, learned picks, and
// follow-ups are kept after their last use. Pinned commands do not expire.
const retentionMonths = 1

// cutoff returns the @cutoff parameter: the time before which unpinned rows
// have expired.
func cutoff() sql.NamedArg {
	return sql.Named("cutoff", time.Now().AddDate(0, -retentionMonths, 0).Unix())
}

type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

// hasher returns the function that turns directories and pick queries into
// the keyed hashes they are stored as. They are only ever compared for
// equality, so they do not need to be stored in plain text.
func hasher(q rowQuerier) (func(string) string, error) {
	var key []byte
	if err := q.QueryRow(`SELECT value FROM settings WHERE name = 'hash_key'`).Scan(&key); err != nil {
		return nil, err
	}

	return func(value string) string { return hashValue(key, value) }, nil
}

// hashValue returns the HMAC-SHA256 of value in hex. An empty value stays
// empty, because it means that there is no value.
func hashValue(key []byte, value string) string {
	if value == "" {
		return ""
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

// hashStoredValues creates the hash key and replaces the plain text
// directories and pick queries stored by earlier versions with their hashes.
func hashStoredValues(ctx context.Context, tx *sql.Tx) error {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE settings (
			name TEXT PRIMARY KEY,
			value BLOB NOT NULL
		)
	`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO settings (name, value) VALUES ('hash_key', ?)`, key); err != nil {
		return err
	}

	for _, column := range []struct{ table, name string }{
		{"command_dirs", "dir"},
		{"command_dirs", "root"},
		{"picks", "query"},
	} {
		if err := rehash(ctx, tx, key, column.table, column.name); err != nil {
			return err
		}
	}

	return nil
}

// rehash replaces every value in column of table with its hash.
func rehash(ctx context.Context, tx *sql.Tx, key []byte, table, column string) error {
	type row struct {
		id    int64
		value string
	}

	rows, err := tx.QueryContext(ctx, "SELECT rowid, "+column+" FROM "+table)
	if err != nil {
		return err
	}

	var found []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.value); err != nil {
			_ = rows.Close()
			return err
		}
		found = append(found, r)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	update := "UPDATE " + table + " SET " + column + " = ? WHERE rowid = ?"
	for _, r := range found {
		if _, err := tx.ExecContext(ctx, update, hashValue(key, r.value), r.id); err != nil {
			return err
		}
	}

	return nil
}
