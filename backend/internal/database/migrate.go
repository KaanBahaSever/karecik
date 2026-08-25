package database

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"karecik/backend/migrations"
)

// Migrate, migrations klasorundeki .sql dosyalarini sirayla uygular.
// Uygulanan her dosya schema_migrations tablosuna yazilir; ikinci calistirmada
// atlanir. Her dosya tek bir transaction icinde calisir.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	if err != nil {
		return fmt.Errorf("schema_migrations tablosu olusturulamadi: %w", err)
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("migration dosyalari okunamadi: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // 001_..., 002_... sirasi korunur

	for _, name := range names {
		if applied[name] {
			continue
		}

		content, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("%s okunamadi: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("transaction baslatilamadi: %w", err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration basarisiz (%s): %w", name, err)
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration kaydedilemedi (%s): %w", name, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("migration commit edilemedi (%s): %w", name, err)
		}

		log.Printf("[karecik] migration uygulandi: %s", name)
	}

	return nil
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("uygulanan migration'lar okunamadi: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}
