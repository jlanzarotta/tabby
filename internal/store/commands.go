package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"tabby/internal/glob"
)

// Add records a successful run of command at location. A zero location
// records the run without directory context.
func Add(db *sql.DB, command string, location Location) error {
	if strings.IndexFunc(command, unicode.IsControl) >= 0 {
		return nil
	}

	command = strings.TrimSpace(command)

	if isTabbyInvocation(command) {
		return nil
	}

	if utf8.RuneCountInString(command) <= 5 {
		return nil
	}

	patterns, err := exclusionPatterns(db)
	if err != nil {
		return err
	}
	if matchesAny(patterns, command) {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	_, err = tx.Exec(`
		INSERT INTO commands (command, use_count, last_used_at)
		VALUES (?, 1, ?)
		ON CONFLICT(command) DO UPDATE SET
			use_count = use_count + 1,
			last_used_at = excluded.last_used_at
	`, command, now)
	if err != nil {
		return err
	}

	if location.Dir != "" {
		_, err = tx.Exec(`
			INSERT INTO command_dirs (command, dir, root, use_count, last_used_at)
			VALUES (?, ?, ?, 1, ?)
			ON CONFLICT(command, dir) DO UPDATE SET
				root = excluded.root,
				use_count = use_count + 1,
				last_used_at = excluded.last_used_at
		`, command, location.Dir, location.Root, now)
		if err != nil {
			return err
		}
	}

	if err := prune(tx); err != nil {
		return err
	}

	return tx.Commit()
}

// prune deletes rows that List and ListLocal no longer return so that they
// do not remain on disk indefinitely. Pinned commands are kept.
func prune(tx *sql.Tx) error {
	_, err := tx.Exec(`
		DELETE FROM commands
		WHERE pinned = 0 AND last_used_at <= unixepoch('now', '-1 month')
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM command_dirs
		WHERE last_used_at <= unixepoch('now', '-1 month')
	`)

	return err
}

// Pin keeps command from expiring, adding it to the store first if needed.
func Pin(db *sql.DB, command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("command must not be empty")
	}
	if strings.IndexFunc(command, unicode.IsControl) >= 0 {
		return fmt.Errorf("command must not contain control characters")
	}

	patterns, err := exclusionPatterns(db)
	if err != nil {
		return err
	}
	if matchesAny(patterns, command) {
		return fmt.Errorf("command matches an exclusion pattern; remove the exclusion first")
	}

	_, err = db.Exec(`
		INSERT INTO commands (command, use_count, last_used_at, pinned)
		VALUES (?, 1, ?, 1)
		ON CONFLICT(command) DO UPDATE SET pinned = 1
	`, command, time.Now().Unix())

	return err
}

// Unpin lets command expire again. It returns an error when command is not
// pinned.
func Unpin(db *sql.DB, command string) error {
	result, err := db.Exec(`
		UPDATE commands SET pinned = 0
		WHERE command = ? AND pinned <> 0
	`, strings.TrimSpace(command))
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("command %q is not pinned", command)
	}

	return nil
}

func isTabbyInvocation(command string) bool {
	fields := strings.Fields(command)
	if len(fields) > 1 && (fields[0] == "command" || fields[0] == "builtin" || fields[0] == "&") {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return false
	}

	name := strings.Trim(fields[0], `"'`)
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSuffix(strings.ToLower(name), ".exe")

	return name == "tabby"
}

func matchesAny(patterns []string, command string) bool {
	lowered := strings.ToLower(command)
	for _, pattern := range patterns {
		if glob.Match(strings.ToLower(pattern), lowered) {
			return true
		}
	}

	return false
}

func exclusionPatterns(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT pattern FROM exclusions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patterns []string

	for rows.Next() {
		var pattern string
		if err := rows.Scan(&pattern); err != nil {
			return nil, err
		}
		patterns = append(patterns, pattern)
	}

	return patterns, rows.Err()
}

func Remove(db *sql.DB, command string) error {
	_, err := db.Exec(`
		DELETE FROM commands
		WHERE command = ?
	`, command)

	return err
}

func List(db *sql.DB) ([]Command, error) {
	rows, err := db.Query(`
		SELECT command, use_count, last_used_at, pinned FROM commands
		WHERE pinned <> 0 OR last_used_at > unixepoch('now', '-1 month')
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commands []Command

	for rows.Next() {
		var command Command

		if err := rows.Scan(
			&command.Text,
			&command.UseCount,
			&command.LastUsed,
			&command.Pinned,
		); err != nil {
			return nil, err
		}

		commands = append(commands, command)
	}

	return commands, rows.Err()
}

// ListLocal returns the recent usage of commands in the directory of
// location and elsewhere in its repository.
func ListLocal(db *sql.DB, location Location) ([]DirUsage, error) {
	if location.Dir == "" {
		return nil, nil
	}

	rows, err := db.Query(`
		SELECT command, dir, use_count, last_used_at FROM command_dirs
		WHERE last_used_at > unixepoch('now', '-1 month')
			AND (dir = ? OR (? <> '' AND root = ?))
	`, location.Dir, location.Root, location.Root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usages []DirUsage

	for rows.Next() {
		var usage DirUsage
		var dir string

		if err := rows.Scan(
			&usage.Command,
			&dir,
			&usage.UseCount,
			&usage.LastUsed,
		); err != nil {
			return nil, err
		}

		usage.SameDir = dir == location.Dir
		usages = append(usages, usage)
	}

	return usages, rows.Err()
}

// AddExclusion stores pattern and deletes every stored command it matches.
// It returns the number of deleted commands.
func AddExclusion(db *sql.DB, pattern string) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
		INSERT INTO exclusions (pattern, created_at)
		VALUES (?, ?)
		ON CONFLICT(pattern) DO NOTHING
	`, pattern, time.Now().Unix())
	if err != nil {
		return 0, err
	}

	rows, err := tx.Query(`SELECT command FROM commands`)
	if err != nil {
		return 0, err
	}

	var matched []string
	patterns := []string{pattern}
	for rows.Next() {
		var command string
		if err := rows.Scan(&command); err != nil {
			_ = rows.Close()
			return 0, err
		}
		if matchesAny(patterns, command) {
			matched = append(matched, command)
		}
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, command := range matched {
		if _, err := tx.Exec(`DELETE FROM commands WHERE command = ?`, command); err != nil {
			return 0, err
		}
	}

	return len(matched), tx.Commit()
}

func RemoveExclusion(db *sql.DB, pattern string) error {
	_, err := db.Exec(`
		DELETE FROM exclusions
		WHERE pattern = ?
	`, pattern)

	return err
}

func ListExclusions(db *sql.DB) ([]Exclusion, error) {
	rows, err := db.Query(`
		SELECT pattern, created_at FROM exclusions
		ORDER BY created_at DESC, pattern
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exclusions []Exclusion

	for rows.Next() {
		var exclusion Exclusion

		if err := rows.Scan(
			&exclusion.Pattern,
			&exclusion.CreatedAt,
		); err != nil {
			return nil, err
		}

		exclusions = append(exclusions, exclusion)
	}

	return exclusions, rows.Err()
}
