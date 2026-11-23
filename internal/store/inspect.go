package store

import (
	"database/sql"
	"os"
)

// Report describes the database for tabby doctor.
type Report struct {
	// Path is the database that Tabby uses now.
	Path string
	// Exists is false until the database is created on first use.
	Exists bool
	// Moving is true when Path is a location that an earlier version used.
	// The database is moved to Target the next time it is opened.
	Moving bool
	Target string
	// Size and LogSize are the sizes of the database and its write-ahead log
	// in bytes.
	Size    int64
	LogSize int64
	// Version is the schema version of the database, and Latest the version
	// that this build of Tabby migrates to.
	Version int64
	Latest  int64
	// Integrity is "ok", or the first problem that SQLite found.
	Integrity string
	// Stats and Exclusions are only set when Version equals Latest.
	Stats      Stats
	Exclusions int
}

// Inspect reports on the database without creating, moving, or migrating it.
func Inspect() (Report, error) {
	var report Report

	target, err := defaultPath()
	if err != nil {
		return report, err
	}

	report.Path, report.Target = target, target
	if legacy := legacyPath(target); legacy != "" {
		report.Path, report.Moving = legacy, true
	}

	info, err := os.Stat(report.Path)
	if os.IsNotExist(err) {
		return report, nil
	}
	if err != nil {
		return report, err
	}
	report.Exists = true
	report.Size = info.Size()
	if info, err := os.Stat(report.Path + "-wal"); err == nil {
		report.LogSize = info.Size()
	}

	db, err := sql.Open("sqlite", report.Path+"?_pragma=busy_timeout(5000)&_pragma=query_only(1)")
	if err != nil {
		return report, err
	}
	defer db.Close()

	provider, err := newProvider(db)
	if err != nil {
		return report, err
	}
	report.Latest = latestVersion(provider)

	// The provider would create the version table, which a read-only
	// connection cannot do, so the version is read directly.
	var tables int
	if err := db.QueryRow(`
		SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'goose_db_version'
	`).Scan(&tables); err != nil {
		return report, err
	}
	if tables > 0 {
		if err := db.QueryRow(`
			SELECT coalesce(max(version_id), 0) FROM goose_db_version WHERE is_applied
		`).Scan(&report.Version); err != nil {
			return report, err
		}
	}

	if err := db.QueryRow(`PRAGMA quick_check(1)`).Scan(&report.Integrity); err != nil {
		return report, err
	}

	if report.Version != report.Latest {
		return report, nil
	}

	if report.Stats, err = ReadStats(db); err != nil {
		return report, err
	}
	err = db.QueryRow(`SELECT count(*) FROM exclusions`).Scan(&report.Exclusions)

	return report, err
}

// RetentionMonths returns how long unpinned commands are kept after their
// last use.
func RetentionMonths() int {
	return retentionMonths
}
