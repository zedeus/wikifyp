// Package migrate applies the server's SQL migrations.
//
// Migrations are plain SQL files named NNNN_description.sql and applied in
// version order; the numeric prefix is the version, so renaming the
// description never re-runs a migration. Each file runs in its own
// transaction together with the bookkeeping insert, so a failing migration
// leaves no trace. Applied versions are recorded in schema_migrations and
// never run twice.
package migrate

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrations embed.FS

// lockKey identifies this package's Postgres advisory lock ("wiki" in hex).
const lockKey = 0x77696b69

// Apply runs all pending embedded migrations. The db must use the pgx driver
// (as everything in this server does): migration files can hold several
// statements, which pgx only accepts over its simple protocol.
func Apply(ctx context.Context, db *sql.DB) error {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return ApplyFS(ctx, db, sub)
}

type migration struct {
	version int
	name    string
}

// ApplyFS runs all pending *.sql migrations from fsys in version order.
func ApplyFS(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	pending, err := load(fsys)
	if err != nil {
		return err
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	defer conn.Close()

	return conn.Raw(func(driverConn any) error {
		sc, ok := driverConn.(*stdlib.Conn)
		if !ok {
			return errors.New("migrate: database connection is not pgx")
		}
		pc := sc.Conn()

		// Serialize with other instances migrating the same database (a
		// rolling deploy starts several at once). The session-level lock
		// outlives ctx cancellation and conn.Close only returns the session
		// to the pool, so the unlock must run even on a canceled ctx; if it
		// fails anyway, close the session so the lock dies with it instead
		// of going back to the pool still held.
		if _, err := pc.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
			return fmt.Errorf("migrate: acquire advisory lock: %w", err)
		}
		defer func() {
			unlockCtx := context.WithoutCancel(ctx)
			if _, err := pc.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", lockKey); err != nil {
				pc.Close(unlockCtx)
			}
		}()

		if _, err := pc.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
			version integer PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
			return fmt.Errorf("migrate: create schema_migrations: %w", err)
		}

		applied, err := appliedVersions(ctx, pc)
		if err != nil {
			return err
		}

		for _, m := range pending {
			if applied[m.version] {
				continue
			}
			body, err := fs.ReadFile(fsys, m.name)
			if err != nil {
				return fmt.Errorf("migrate: read %s: %w", m.name, err)
			}
			if err := applyOne(ctx, pc, m.version, string(body)); err != nil {
				return fmt.Errorf("migrate: apply %s: %w", m.name, err)
			}
		}
		return nil
	})
}

// load lists the migrations in fsys, validating names and version numbers.
func load(fsys fs.FS) ([]migration, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	seen := map[int]string{}
	ms := make([]migration, 0, len(names))
	for _, name := range names {
		prefix, _, ok := strings.Cut(name, "_")
		version, err := strconv.Atoi(prefix)
		if !ok || err != nil || version < 0 {
			return nil, fmt.Errorf("migrate: %s: migrations are named NNNN_description.sql", name)
		}
		if prev, dup := seen[version]; dup {
			return nil, fmt.Errorf("migrate: %s and %s share version %d", prev, name, version)
		}
		seen[version] = name
		ms = append(ms, migration{version: version, name: name})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].version < ms[j].version })
	return ms, nil
}

func appliedVersions(ctx context.Context, pc *pgx.Conn) (map[int]bool, error) {
	rows, err := pc.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("migrate: read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("migrate: scan version: %w", err)
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func applyOne(ctx context.Context, pc *pgx.Conn, version int, body string) error {
	tx, err := pc.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Exec without arguments uses the simple protocol, so a migration file
	// may contain any number of statements.
	if _, err := tx.Exec(ctx, body); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
