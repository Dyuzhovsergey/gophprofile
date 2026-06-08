// Package main запускает миграции PostgreSQL для GophProfile.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const (
	defaultMigrationsDir = "/app/migrations"
	defaultPingTimeout   = 30 * time.Second
)

// main читает конфигурацию из env и запускает миграции БД.
func main() {
	if err := run(); err != nil {
		slog.Error("failed to run migrations", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("migrations completed successfully")
}

// run подключается к PostgreSQL и применяет все новые миграции.
func run() error {
	dsn := os.Getenv("GOPHPROFILE_DATABASE_DSN")
	if dsn == "" {
		return errors.New("GOPHPROFILE_DATABASE_DSN is empty")
	}

	migrationsDir := os.Getenv("GOPHPROFILE_MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = defaultMigrationsDir
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), defaultPingTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto"); err != nil {
		return err
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	return goose.Up(db, migrationsDir)
}
