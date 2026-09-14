package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Advisory locks of the structural writers.
//
// Row locks alone cannot keep a write into a parent apart from a delete of that
// parent. A delete locks the rows it removes in a fixed order, but a write
// reaches its parent only through the foreign key check of its own INSERT or
// UPDATE — after it has locked the row it moves, when it moves one — and a
// product write that also changes the price then locks the menu row through the
// trigger of migration 010. Depending on where the write lands between the lock
// steps of the delete, the two can end up each holding a row the other one
// needs:
//
//   - a category moved into, or created in, a menu that is being deleted,
//     followed by a price edit of a product in it;
//   - a product moved into, or created in, a category of a menu that is being
//     deleted, between the delete's product and category lock steps, followed
//     by a price edit of that product or a bulk price update of the menu;
//   - a product moved into, or created in, a category that is being deleted,
//     followed by a reorder or a bulk price update that locks it together with
//     a product the delete already holds.
//
// So these writers also take a transaction-level advisory lock on the parent,
// BEFORE any row lock of their transaction:
//
//   - DeleteMenu takes the lock of its menu exclusively, DeleteCategory the lock
//     of its category;
//   - a write into a menu takes that menu's lock shared (writeIntoMenu):
//     CreateCategory, and UpdateCategory when the update names a menu;
//   - a write into a category takes the lock of the category's menu and then
//     the lock of the category, both shared (writeIntoCategory): CreateProduct,
//     ReorderProducts, and UpdateProduct when the update names a category.
//     Every one of them takes the two in that order, the menu's first.
//
// A write into a parent that is being deleted therefore waits for the delete
// before it has locked anything, and then finds the parent gone; a delete of a
// parent that a write is writing into waits for the write before the delete has
// locked anything. Shared locks do not conflict with each other, so creates,
// moves and reorders into the same parent still run side by side. No lock cycle
// can pass through these locks: every transaction takes them before its first
// row lock, so a transaction that holds a row lock never waits for one of them.
//
// Price edits without a move, bulk price updates, category reorders, deletes of
// a single product and settings saves take none of them: none of those puts a
// record into a menu or a category.
//
// The first key is a fixed namespace per kind of parent and the second is
// hashtext of the id; pg_locks lists such a lock with locktype advisory, the
// namespace as classid and the hash as objid. Two ids whose hashes collide share
// a key: they then wait on each other when they need not, but no wait that is
// needed is ever skipped. A lock is only taken on a menu or a category the
// business owns — writeIntoMenu, writeIntoCategory and both deletes read that
// first — so a request of another tenant never waits on one, short of such a
// collision.

// The namespaces, spelled in ASCII: "KRMN" for menus, "KRCT" for categories.
const (
	MenuLockNamespace     int32 = 0x4b524d4e
	CategoryLockNamespace int32 = 0x4b524354
)

// lockMenu takes the advisory lock of a menu until the transaction ends:
// exclusively for a delete of the menu, shared for a write into it.
func lockMenu(ctx context.Context, tx pgx.Tx, menuID uuid.UUID, exclusive bool) error {
	return advisoryLock(ctx, tx, MenuLockNamespace, menuID, exclusive)
}

// lockCategory is lockMenu for a category.
func lockCategory(ctx context.Context, tx pgx.Tx, categoryID uuid.UUID, exclusive bool) error {
	return advisoryLock(ctx, tx, CategoryLockNamespace, categoryID, exclusive)
}

// advisoryLock runs the lock as a statement of its own. Under READ COMMITTED
// every later statement of the transaction then takes its snapshot once the lock
// is held, and so sees whatever the writer it waited for committed.
func advisoryLock(ctx context.Context, tx pgx.Tx, namespace int32, id uuid.UUID, exclusive bool) error {
	function := "pg_advisory_xact_lock_shared"
	if exclusive {
		function = "pg_advisory_xact_lock"
	}
	_, err := tx.Exec(ctx, `SELECT `+function+`($1::int4, hashtext($2::uuid::text))`, namespace, id)
	return err
}

// menuOwned reports whether the business owns the menu. It locks nothing; it is
// what keeps a request from taking the lock of another tenant's menu.
func menuOwned(ctx context.Context, db DB, menuID, businessID uuid.UUID) (bool, error) {
	var owned bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM menus WHERE id = $1 AND business_id = $2)`,
		menuID, businessID).Scan(&owned)
	return owned, err
}

// categoryOwned is menuOwned for a category.
func categoryOwned(ctx context.Context, db DB, categoryID, businessID uuid.UUID) (bool, error) {
	var owned bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM categories WHERE id = $1 AND business_id = $2)`,
		categoryID, businessID).Scan(&owned)
	return owned, err
}

// requireMenu is menuOwned for a caller that is about to write into the menu:
// ErrParentNotFound when the business has no such menu.
func requireMenu(ctx context.Context, db DB, menuID, businessID uuid.UUID) error {
	owned, err := menuOwned(ctx, db, menuID, businessID)
	if err != nil {
		return err
	}
	if !owned {
		return ErrParentNotFound
	}
	return nil
}

// categoryMenu reads the menu a category of the business sits on. It answers
// ErrParentNotFound when the business has no such category, because every
// caller is about to write into it.
func categoryMenu(ctx context.Context, db DB, categoryID, businessID uuid.UUID) (uuid.UUID, error) {
	var menuID uuid.UUID
	err := db.QueryRow(ctx,
		`SELECT menu_id FROM categories WHERE id = $1 AND business_id = $2`,
		categoryID, businessID).Scan(&menuID)
	if isNoRows(err) {
		return uuid.Nil, ErrParentNotFound
	}
	return menuID, err
}

// writeIntoMenu runs write in a transaction that already holds the shared
// advisory lock of a menu of the business.
//
// The menu is checked twice. Before the lock, so that a request never takes —
// or waits on — the lock of a menu the business does not own; and again once
// the lock is held, because a delete that held it may have removed the menu in
// the meantime. ErrParentNotFound comes back from either check, with nothing
// written.
func writeIntoMenu(ctx context.Context, db TxDB, businessID, menuID uuid.UUID,
	write func(tx pgx.Tx) error) error {

	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if err := requireMenu(ctx, tx, menuID, businessID); err != nil {
			return err
		}
		if err := lockMenu(ctx, tx, menuID, false); err != nil {
			return err
		}
		if err := requireMenu(ctx, tx, menuID, businessID); err != nil {
			return err
		}
		return write(tx)
	})
}

// writeIntoCategoryRuns bounds how often writeIntoCategory starts its
// transaction over because the category moved to another menu in between.
const writeIntoCategoryRuns = 3

// errCategoryMoved rolls back a run of writeIntoCategory whose category moved
// to another menu between being read and being locked.
var errCategoryMoved = errors.New("the category moved to another menu while its locks were taken")

// writeIntoCategory runs write in a transaction that already holds the shared
// advisory locks of a category of the business and of its menu, menu first.
//
// The menu has to be read before its lock can be taken, so the category can
// move to another menu in between. The lock would then belong to a menu the
// category has left, and a delete of the menu it moved to would not wait for
// this write. The menu is therefore read again once both locks are held, and a
// run that finds it changed rolls back and starts over. The last run writes
// under the locks it holds whatever it reads: getting there takes a category
// that moves between menus on every run, and RetryOnConflict remains the net
// for any lock conflict that follows from it.
//
// ErrParentNotFound comes back when the category is not there — before the
// locks, or after them because a delete that held its lock removed it.
func writeIntoCategory(ctx context.Context, db TxDB, businessID, categoryID uuid.UUID,
	write func(tx pgx.Tx) error) error {

	for run := 1; ; run++ {
		err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			menuID, err := categoryMenu(ctx, tx, categoryID, businessID)
			if err != nil {
				return err
			}
			if err := lockMenu(ctx, tx, menuID, false); err != nil {
				return err
			}
			if err := lockCategory(ctx, tx, categoryID, false); err != nil {
				return err
			}
			current, err := categoryMenu(ctx, tx, categoryID, businessID)
			if err != nil {
				return err
			}
			if current != menuID && run < writeIntoCategoryRuns {
				return errCategoryMoved
			}
			return write(tx)
		})
		if errors.Is(err, errCategoryMoved) {
			continue
		}
		return err
	}
}
