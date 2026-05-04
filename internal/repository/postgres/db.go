package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrEmptyDatabaseDSN = errors.New("database dsn is empty")

const defaultPingTimeout = 5 * time.Second

// NewPool создаёт пул подключений к PostgreSQL и проверяет соединение.
func NewPool(ctx context.Context, databaseDSN string) (*pgxpool.Pool, error) {
	if databaseDSN == "" {
		return nil, ErrEmptyDatabaseDSN
	}

	pool, err := pgxpool.New(ctx, databaseDSN)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
