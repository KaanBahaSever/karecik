package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect, PostgreSQL baglanti havuzunu acar ve baglantiyi dogrular.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL cozulemedi (bicim: postgres://kullanici:sifre@host:5432/veritabani?sslmode=disable): %w", err)
	}

	poolCfg.MaxConns = 10
	poolCfg.MinConns = 1
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("baglanti havuzu olusturulamadi: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf(
			"veritabanina baglanilamadi: %w\n"+
				"  Kontrol et:\n"+
				"    1) PostgreSQL servisi calisiyor mu? (Windows > Hizmetler > postgresql-x64-16)\n"+
				"    2) 'karecik' veritabani olusturuldu mu? (psql -U postgres -c \"CREATE DATABASE karecik;\")\n"+
				"    3) .env icindeki DATABASE_URL sifresi dogru mu?", err)
	}

	log.Printf("[karecik] veritabanina baglanildi (%s:%d/%s)",
		poolCfg.ConnConfig.Host, poolCfg.ConnConfig.Port, poolCfg.ConnConfig.Database)

	return pool, nil
}
