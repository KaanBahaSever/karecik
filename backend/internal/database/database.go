package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens the PostgreSQL connection pool and verifies the connection.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse DATABASE_URL (expected postgres://user:password@host:5432/database?sslmode=disable): %w", err)
	}

	poolCfg.MaxConns = 10
	poolCfg.MinConns = 1
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("could not create the connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf(
			"could not connect to the database: %w\n"+
				"  Check that:\n"+
				"    1) the PostgreSQL service is running (Windows > Services > postgresql-18)\n"+
				"    2) the 'karecik' database exists (psql -U postgres -c \"CREATE DATABASE karecik;\")\n"+
				"    3) the password inside DATABASE_URL in .env is correct", err)
	}

	log.Printf("[karecik] connected to the database (%s:%d/%s)",
		poolCfg.ConnConfig.Host, poolCfg.ConnConfig.Port, poolCfg.ConnConfig.Database)

	return pool, nil
}
