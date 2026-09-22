package store

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations reads migration files from the provided embed.FS and applies any
// that haven't been applied yet. Uses a schema_migrations tracking table for idempotency.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsFS embed.FS, logger *slog.Logger) error {
	// Ensure the schema_migrations tracking table exists.
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("reading embedded migrations: %w", err)
	}

	// Collect only *.up.sql files and sort them by filename for ordered execution.
	var upFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	for _, filename := range upFiles {
		// Extract version: "000001_create_jobs.up.sql" -> "000001_create_jobs"
		version := strings.TrimSuffix(filename, ".up.sql")

		// Check if this migration was already applied.
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`,
			version,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("checking migration %s: %w", version, err)
		}
		if exists {
			continue
		}

		content, err := migrationsFS.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("reading migration file %s: %w", filename, err)
		}

		logger.Info("applying migration", "version", version, "file", filename)
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("applying migration %s: %w", version, err)
		}

		if _, err := pool.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`, version,
		); err != nil {
			return fmt.Errorf("recording migration %s: %w", version, err)
		}
	}

	logger.Info("database migrations complete")
	return nil
}
