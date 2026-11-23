package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// goMigrations are the migrations that cannot be written in SQL.
var goMigrations = []*goose.Migration{
	goose.NewGoMigration(8, &goose.GoFunc{RunTx: hashStoredValues}, nil),
}

// newProvider returns the migration provider for db. Creating it does not
// touch the database.
func newProvider(db *sql.DB) (*goose.Provider, error) {
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}

	return goose.NewProvider(goose.DialectSQLite3, db, files, goose.WithGoMigrations(goMigrations...))
}

func latestVersion(provider *goose.Provider) int64 {
	sources := provider.ListSources()
	return sources[len(sources)-1].Version
}

func migrate(db *sql.DB) error {
	ctx := context.Background()
	provider, err := newProvider(db)
	if err != nil {
		return err
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return err
	}
	latest := latestVersion(provider)
	if version > latest {
		return fmt.Errorf("database schema version %d is newer than supported version %d; upgrade Tabby", version, latest)
	}
	if version == latest {
		return nil
	}
	if _, err = provider.Up(ctx); err != nil {
		return err
	}

	// Migrations can delete or rewrite stored values, so the old ones are
	// cleared from the file afterward.
	return scrub(db)
}
