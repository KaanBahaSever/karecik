package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"karecik/backend/internal/models"
	"karecik/backend/internal/utils"
)

const userColumns = `id, email, password_hash, business_name, role, created_at, updated_at`

func scanUser(row pgx.Row) (*models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.BusinessName,
		&u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// GetUserByEmail, e-postayi buyuk/kucuk harf duyarsiz arar.
func GetUserByEmail(ctx context.Context, db DB, email string) (*models.User, error) {
	row := db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1)`,
		strings.TrimSpace(email))
	return scanUser(row)
}

// GetUserByID, kimlige gore kullaniciyi getirir.
func GetUserByID(ctx context.Context, db DB, id uuid.UUID) (*models.User, error) {
	row := db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	return scanUser(row)
}

// EmailExists, e-postanin kayitli olup olmadigini soyler.
func EmailExists(ctx context.Context, db DB, email string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE lower(email) = lower($1))`,
		strings.TrimSpace(email)).Scan(&exists)
	return exists, err
}

// CreateAccount, kullanici + isletme kaydini tek transaction'da olusturur.
// Slug isletme adindan uretilir; cakisirsa sonuna -2, -3 ... eklenir.
func CreateAccount(ctx context.Context, pool *pgxpool.Pool,
	businessName, email, passwordHash string) (*models.User, *models.Business, error) {

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var user models.User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, business_name)
		VALUES ($1, $2, $3)
		RETURNING `+userColumns,
		strings.TrimSpace(email), passwordHash, strings.TrimSpace(businessName),
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.BusinessName,
		&user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, nil, ErrDuplicate
		}
		return nil, nil, fmt.Errorf("kullanici olusturulamadi: %w", err)
	}

	slug, err := uniqueSlug(ctx, tx, utils.Slugify(businessName))
	if err != nil {
		return nil, nil, err
	}

	business, err := insertBusiness(ctx, tx, user.ID, strings.TrimSpace(businessName), slug)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("kayit tamamlanamadi: %w", err)
	}

	return &user, business, nil
}

// uniqueSlug, bos olan ilk slug varyantini dondurur: kahve, kahve-2, kahve-3 ...
func uniqueSlug(ctx context.Context, db DB, base string) (string, error) {
	if utils.IsReservedSlug(base) {
		base += "-menu"
	}

	candidate := base
	for i := 2; i < 200; i++ {
		var exists bool
		err := db.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM businesses WHERE slug = $1)`, candidate).Scan(&exists)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	return "", fmt.Errorf("uygun bir menü adresi üretilemedi")
}
