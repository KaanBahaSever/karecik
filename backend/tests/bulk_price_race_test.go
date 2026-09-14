package tests

// Writes that run into each other on the same rows must not lose anything: the
// bulk price update, the product reorder and the category reorder.
//
// Each locks its rows in one statement and writes them in a later one, inside
// one transaction. Every case below stages the race deterministically with
// transactions of the test's own: they hold product rows, another writer runs
// while the one under test waits, and the test moves on only once PostgreSQL
// reports who is waiting for whom.

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"karecik/backend/internal/utils"
)

// bulkPriceAnswer is the part of the bulk price answer these cases read.
type bulkPriceAnswer struct {
	Applied  bool `json:"applied"`
	Affected int  `json:"affected"`
	Preview  []struct {
		ID       string  `json:"id"`
		OldPrice float64 `json:"old_price"`
		NewPrice float64 `json:"new_price"`
	} `json:"preview"`
}

// raisedByTenPercent is the price a +10% update without rounding gives.
func raisedByTenPercent(price float64) float64 {
	return utils.RoundPrice(utils.ApplyPercentage(price, 10), utils.RoundNone)
}

// samePrice compares two prices at the precision of the column.
func samePrice(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

// TestBulkPriceRaisesEveryProductWhileOtherWritersTouchThem is the lost update
// of the bulk price endpoint, staged exactly.
//
// Eight products; the test holds the rows with the lowest and the highest id.
// The apply starts and waits on the lowest one. Meanwhile the six products in
// between are written — an UPDATE that sets position to itself, which changes
// nothing but still makes a new version of each row — and committed. Then the
// two held rows are released one after the other. Every one of the eight
// prices has to be raised, and the answer has to say 8.
func TestBulkPriceRaisesEveryProductWhileOtherWritersTouchThem(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Toplu Fiyat Kafe", "bulk-race-owner@example.test")
	menu := h.createMenu(owner, "Toplu Fiyat Menü")
	category := h.createCategory(owner, menu.ID, "Kahveler")

	before := make(map[string]float64)
	ids := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		product := h.createProduct(owner, category.ID, fmt.Sprintf("Kahve %d", i+1), float64(60+i))
		before[product.ID] = float64(60 + i)
		ids = append(ids, product.ID)
	}
	ids = idOrder(t, h, ids)

	var pending []*pendingRequest
	pend := func() []*pendingRequest { return pending }
	lowest := holdLock(t, h, pend, `SELECT id FROM products WHERE id = $1::text::uuid FOR UPDATE`, ids[0])
	highest := holdLock(t, h, pend, `SELECT id FROM products WHERE id = $1::text::uuid FOR UPDATE`, ids[7])

	apply := h.startRequest(http.MethodPost, "/api/products/bulk-price", owner.session, map[string]any{
		"menu_id": menu.ID, "percentage": 10, "rounding": "none", "apply": true,
	})
	pending = append(pending, apply)
	waiter := waitForWaiterBlockedBy(t, h, "the bulk apply at the lowest product", lowest.pid)
	t.Logf("the apply (backend %d) waits on the lowest product while running: %s", waiter.pid, waiter.query)

	ctx, cancel := context.WithTimeout(context.Background(), lockWaitTimeout)
	defer cancel()
	tag, err := h.pool.Exec(ctx,
		`UPDATE products SET position = position WHERE id = ANY($1::text[]::uuid[])`, ids[1:7])
	if err != nil {
		t.Fatalf("could not write the six products in between: %v", err)
	}
	if tag.RowsAffected() != 6 {
		t.Fatalf("fixture: the write in between touched %d products, want 6", tag.RowsAffected())
	}

	if err := lowest.tx.Commit(context.Background()); err != nil {
		t.Fatalf("could not release the lowest product: %v", err)
	}
	waiter = waitForWaiterBlockedBy(t, h, "the bulk apply at the highest product", highest.pid)
	t.Logf("the apply (backend %d) now waits on the highest product", waiter.pid)
	if err := highest.tx.Commit(context.Background()); err != nil {
		t.Fatalf("could not release the highest product: %v", err)
	}

	status, body := apply.wait(t, "POST /api/products/bulk-price")
	if status != http.StatusOK {
		t.Fatalf("POST /api/products/bulk-price answered %d, want 200: %s", status, body)
	}
	var answer bulkPriceAnswer
	decodeInto(t, "POST /api/products/bulk-price", body, &answer)
	if !answer.Applied || answer.Affected != 8 {
		t.Errorf("the apply answered applied=%t affected=%d, want true and 8: %s",
			answer.Applied, answer.Affected, shorten(body))
	}

	raised := 0
	for _, id := range ids {
		_, _, stored := productRow(t, h, id)
		if samePrice(stored, raisedByTenPercent(before[id])) {
			raised++
		} else {
			t.Errorf("product %s stores %.2f, want %.2f (+10%% of %.2f)", id, stored,
				raisedByTenPercent(before[id]), before[id])
		}
	}
	if raised != 8 {
		t.Errorf("%d of 8 prices were raised", raised)
	}

	// The answer describes what was written, row by row.
	if len(answer.Preview) != 8 {
		t.Fatalf("the answer lists %d products, want 8: %s", len(answer.Preview), shorten(body))
	}
	for _, row := range answer.Preview {
		_, _, stored := productRow(t, h, row.ID)
		if !samePrice(row.OldPrice, before[row.ID]) || !samePrice(row.NewPrice, stored) {
			t.Errorf("the answer says %s went from %.2f to %.2f; it was %.2f and stores %.2f",
				row.ID, row.OldPrice, row.NewPrice, before[row.ID], stored)
		}
	}
}

// TestBulkPriceAppliesThePercentageToAPriceEditedWhileItWaits: an inline price
// edit commits while the apply waits for its row locks, so the apply has to
// raise the NEW price — and say so in its answer.
func TestBulkPriceAppliesThePercentageToAPriceEditedWhileItWaits(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Güncel Fiyat Kafe", "bulk-lost-update-owner@example.test")
	menu := h.createMenu(owner, "Güncel Fiyat Menü")
	category := h.createCategory(owner, menu.ID, "Tatlılar")

	before := make(map[string]float64)
	ids := make([]string, 0, 3)
	for i, price := range []float64{100, 200, 300} {
		product := h.createProduct(owner, category.ID, fmt.Sprintf("Tatlı %d", i+1), price)
		before[product.ID] = price
		ids = append(ids, product.ID)
	}
	ids = idOrder(t, h, ids)
	edited := ids[2]
	const editedPrice = 500.0

	var pending []*pendingRequest
	held := holdLock(t, h, func() []*pendingRequest { return pending },
		`SELECT id FROM products WHERE id = $1::text::uuid FOR UPDATE`, ids[0])

	apply := h.startRequest(http.MethodPost, "/api/products/bulk-price", owner.session, map[string]any{
		"menu_id": menu.ID, "percentage": 10, "rounding": "none", "apply": true,
	})
	pending = append(pending, apply)
	waiter := waitForWaiterBlockedBy(t, h, "the bulk apply", held.pid)
	t.Logf("the apply (backend %d) waits on the lowest product while running: %s", waiter.pid, waiter.query)

	// The edited product has the highest id, so the apply has not locked it yet
	// and the edit goes through at once.
	edit := h.startRequest(http.MethodPatch, "/api/products/"+edited+"/price", owner.session,
		map[string]any{"price": editedPrice})
	pending = append(pending, edit)
	select {
	case <-edit.done:
	case <-time.After(lockWaitTimeout):
		t.Fatalf("the inline price edit did not answer while the apply was waiting")
	}
	if status, body := edit.wait(t, "PATCH /api/products/:id/price"); status != http.StatusOK {
		t.Fatalf("PATCH /api/products/:id/price answered %d, want 200: %s", status, body)
	}

	if err := held.tx.Commit(context.Background()); err != nil {
		t.Fatalf("could not release the held product: %v", err)
	}
	status, body := apply.wait(t, "POST /api/products/bulk-price")
	if status != http.StatusOK {
		t.Fatalf("POST /api/products/bulk-price answered %d, want 200: %s", status, body)
	}

	want := map[string]float64{
		ids[0]: raisedByTenPercent(before[ids[0]]),
		ids[1]: raisedByTenPercent(before[ids[1]]),
		edited: raisedByTenPercent(editedPrice),
	}
	for _, id := range ids {
		if _, _, stored := productRow(t, h, id); !samePrice(stored, want[id]) {
			t.Errorf("product %s stores %.2f, want %.2f", id, stored, want[id])
		}
	}

	var answer bulkPriceAnswer
	decodeInto(t, "POST /api/products/bulk-price", body, &answer)
	if answer.Affected != 3 {
		t.Errorf("the apply answered affected=%d, want 3", answer.Affected)
	}
	found := false
	for _, row := range answer.Preview {
		if row.ID != edited {
			continue
		}
		found = true
		if !samePrice(row.OldPrice, editedPrice) || !samePrice(row.NewPrice, raisedByTenPercent(editedPrice)) {
			t.Errorf("the answer says the edited product went from %.2f to %.2f, want %.2f to %.2f",
				row.OldPrice, row.NewPrice, editedPrice, raisedByTenPercent(editedPrice))
		}
	}
	if !found {
		t.Errorf("the answer does not list the edited product: %s", shorten(body))
	}
}

// TestConcurrentProductReordersLeaveDistinctPositions: two reorders of one
// category run into each other.
//
// Six products. The test holds the one with the lowest id, and the first
// reorder — all six, in a new order that puts that product first — waits on
// it. A second reorder of the four products with the next ids goes through
// meanwhile. When the held row is released the first reorder finishes, and
// since it lists all six products and finished last, its order is the one the
// category ends up in: six distinct positions, 0 to 5.
func TestConcurrentProductReordersLeaveDistinctPositions(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Sıralama Yarışı Kafe", "reorder-race-owner@example.test")
	menu := h.createMenu(owner, "Sıralama Menü")
	category := h.createCategory(owner, menu.ID, "Soğuk İçecekler")

	ids := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		ids = append(ids, h.createProduct(owner, category.ID, fmt.Sprintf("İçecek %d", i+1), 50).ID)
	}
	ids = idOrder(t, h, ids)

	full := []string{ids[0], ids[5], ids[4], ids[3], ids[2], ids[1]}
	subset := []string{ids[4], ids[3], ids[2], ids[1]}

	var pending []*pendingRequest
	held := holdLock(t, h, func() []*pendingRequest { return pending },
		`SELECT id FROM products WHERE id = $1::text::uuid FOR UPDATE`, ids[0])

	first := h.startRequest(http.MethodPut, "/api/products/reorder", owner.session,
		map[string]any{"category_id": category.ID, "ids": full})
	pending = append(pending, first)
	waiter := waitForWaiterBlockedBy(t, h, "the full reorder", held.pid)
	t.Logf("the full reorder (backend %d) waits on the held product while running: %s", waiter.pid, waiter.query)

	second := h.startRequest(http.MethodPut, "/api/products/reorder", owner.session,
		map[string]any{"category_id": category.ID, "ids": subset})
	pending = append(pending, second)
	select {
	case <-second.done:
	case <-time.After(lockWaitTimeout):
		t.Fatalf("the reorder of the four products did not answer while the full reorder was waiting")
	}
	if status, body := second.wait(t, "the reorder of four products"); status != http.StatusOK {
		t.Fatalf("the reorder of four products answered %d, want 200: %s", status, body)
	}

	if err := held.tx.Commit(context.Background()); err != nil {
		t.Fatalf("could not release the held product: %v", err)
	}
	if status, body := first.wait(t, "the full reorder"); status != http.StatusOK {
		t.Fatalf("the full reorder answered %d, want 200: %s", status, body)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := h.pool.Query(ctx,
		`SELECT id::text, position FROM products WHERE category_id = $1::text::uuid`, category.ID)
	if err != nil {
		t.Fatalf("could not read the positions: %v", err)
	}
	positions := make(map[string]int)
	for rows.Next() {
		var id string
		var position int
		if err := rows.Scan(&id, &position); err != nil {
			t.Fatalf("could not read the positions: %v", err)
		}
		positions[id] = position
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("could not read the positions: %v", err)
	}

	seen := make(map[int][]string)
	for id, position := range positions {
		seen[position] = append(seen[position], id)
	}
	var duplicates []string
	for position, holders := range seen {
		if len(holders) > 1 {
			sort.Strings(holders)
			duplicates = append(duplicates, fmt.Sprintf("%d: %s", position, strings.Join(holders, ", ")))
		}
	}
	sort.Strings(duplicates)
	if len(duplicates) > 0 {
		t.Errorf("the category holds duplicate positions after the two reorders: %v (all: %v)", duplicates, positions)
	}
	for index, id := range full {
		if positions[id] != index {
			t.Errorf("product %s is at position %d, want %d from the reorder that finished last (all: %v)",
				id, positions[id], index, positions)
		}
	}
}

// TestConcurrentCategoryReordersLeaveDistinctPositions is the same race for
// PUT /api/categories/reorder: six categories, the one with the lowest id held,
// a reorder of all six waiting on it, and a reorder of the four with the next
// ids going through meanwhile.
func TestConcurrentCategoryReordersLeaveDistinctPositions(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Kategori Sırası Kafe", "category-reorder-race-owner@example.test")
	menu := h.createMenu(owner, "Kategori Sırası Menü")

	ids := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		ids = append(ids, h.createCategory(owner, menu.ID, fmt.Sprintf("Kategori %d", i+1)).ID)
	}
	// uuid values compare byte by byte, which is the order of their lowercase
	// hexadecimal spelling.
	sort.Strings(ids)

	full := []string{ids[0], ids[5], ids[4], ids[3], ids[2], ids[1]}
	subset := []string{ids[4], ids[3], ids[2], ids[1]}

	var pending []*pendingRequest
	held := holdLock(t, h, func() []*pendingRequest { return pending },
		`SELECT id FROM categories WHERE id = $1::text::uuid FOR UPDATE`, ids[0])

	first := h.startRequest(http.MethodPut, "/api/categories/reorder", owner.session, map[string]any{"ids": full})
	pending = append(pending, first)
	waiter := waitForWaiterBlockedBy(t, h, "the full category reorder", held.pid)
	t.Logf("the full reorder (backend %d) waits on the held category while running: %s", waiter.pid, waiter.query)

	second := h.startRequest(http.MethodPut, "/api/categories/reorder", owner.session, map[string]any{"ids": subset})
	pending = append(pending, second)
	select {
	case <-second.done:
	case <-time.After(lockWaitTimeout):
		t.Fatalf("the reorder of the four categories did not answer while the full reorder was waiting")
	}
	if status, body := second.wait(t, "the reorder of four categories"); status != http.StatusOK {
		t.Fatalf("the reorder of four categories answered %d, want 200: %s", status, body)
	}

	if err := held.tx.Commit(context.Background()); err != nil {
		t.Fatalf("could not release the held category: %v", err)
	}
	if status, body := first.wait(t, "the full category reorder"); status != http.StatusOK {
		t.Fatalf("the full category reorder answered %d, want 200: %s", status, body)
	}

	positions := make(map[string]int, len(ids))
	seen := make(map[int]string, len(ids))
	for _, id := range ids {
		position := categoryPosition(t, h, id)
		positions[id] = position
		if other, taken := seen[position]; taken {
			t.Errorf("categories %s and %s share position %d after the two reorders", other, id, position)
		}
		seen[position] = id
	}
	for index, id := range full {
		if positions[id] != index {
			t.Errorf("category %s is at position %d, want %d from the reorder that finished last (all: %v)",
				id, positions[id], index, positions)
		}
	}
}

// TestBulkPriceRaisesAProductMovedWithinTheMenuWhileItWaits: an apply on a
// whole menu waits for one product row, and meanwhile another product of the
// menu is moved to a second category of the same menu — through the product
// dialog, or by a drag and drop — and commits. That product is still on the
// menu when the apply locks and writes it, so it has to be raised and listed
// like every other.
//
// A locking SELECT that reached the products through a join to categories
// would check the moved row again against the category row it read first, and
// under READ COMMITTED it would skip the row without an error.
func TestBulkPriceRaisesAProductMovedWithinTheMenuWhileItWaits(t *testing.T) {
	h := newHarness(t)
	owner := h.register("owner", "Menü İçi Taşıma Kafe", "bulk-move-owner@example.test")

	for _, variant := range []string{"move_by_product_dialog", "move_by_reorder"} {
		t.Run(variant, func(t *testing.T) {
			menu := h.createMenu(owner, "Taşıma "+variant)
			first := h.createCategory(owner, menu.ID, "Birinci")
			second := h.createCategory(owner, menu.ID, "İkinci")
			ids := make([]string, 0, 4)
			for i := 0; i < 4; i++ {
				ids = append(ids, h.createProduct(owner, first.ID, fmt.Sprintf("Ürün %d", i+1), 100).ID)
			}
			ids = idOrder(t, h, ids)
			// The moved product sorts after the held one, so the apply has not
			// locked it yet while it waits.
			held, moved := ids[0], ids[2]

			deadlocksBefore := deadlocksSoFar(t, h)

			var pending []*pendingRequest
			lock := holdLock(t, h, func() []*pendingRequest { return pending },
				`SELECT id FROM products WHERE id = $1::text::uuid FOR UPDATE`, held)

			apply := h.startRequest(http.MethodPost, "/api/products/bulk-price", owner.session, map[string]any{
				"menu_id": menu.ID, "percentage": 10, "rounding": "none", "apply": true,
			})
			pending = append(pending, apply)
			waiter := waitForWaiterBlockedBy(t, h, "the bulk apply", lock.pid)
			t.Logf("the apply (backend %d) waits on the held product while running: %s", waiter.pid, waiter.query)

			var move *pendingRequest
			if variant == "move_by_product_dialog" {
				move = h.startRequest(http.MethodPut, "/api/products/"+moved, owner.session,
					map[string]any{"category_id": second.ID})
			} else {
				move = h.startRequest(http.MethodPut, "/api/products/reorder", owner.session,
					map[string]any{"category_id": second.ID, "ids": []string{moved}})
			}
			pending = append(pending, move)
			requireAnswersWithin(t, "the move within the menu", move, lockWaitTimeout)
			if status, body := move.wait(t, variant); status != http.StatusOK {
				t.Fatalf("%s answered %d while the apply waited, want 200: %s", variant, status, body)
			}

			if err := lock.tx.Commit(context.Background()); err != nil {
				t.Fatalf("could not release the held product: %v", err)
			}
			status, body := apply.wait(t, "POST /api/products/bulk-price")
			if status != http.StatusOK {
				t.Fatalf("POST /api/products/bulk-price answered %d, want 200: %s", status, body)
			}

			var answer bulkPriceAnswer
			decodeInto(t, "POST /api/products/bulk-price", body, &answer)
			if answer.Affected != 4 || len(answer.Preview) != 4 {
				t.Errorf("the apply answered affected=%d with %d rows, want 4 and 4: %s",
					answer.Affected, len(answer.Preview), shorten(body))
			}
			listed := false
			for _, row := range answer.Preview {
				if row.ID == moved {
					listed = true
					if !samePrice(row.OldPrice, 100) || !samePrice(row.NewPrice, 110) {
						t.Errorf("the answer says the moved product went from %.2f to %.2f, want 100.00 to 110.00",
							row.OldPrice, row.NewPrice)
					}
				}
			}
			if !listed {
				t.Errorf("the answer does not list the product moved within the menu: %s", shorten(body))
			}
			for _, id := range ids {
				if _, _, stored := productRow(t, h, id); !samePrice(stored, 110) {
					t.Errorf("product %s stores %.2f, want 110.00", id, stored)
				}
			}
			if _, category, _ := productRow(t, h, moved); category != second.ID {
				t.Errorf("the moved product sits in category %s, want %s", category, second.ID)
			}
			if deadlocks := deadlocksSoFar(t, h) - deadlocksBefore; deadlocks != 0 {
				t.Errorf("PostgreSQL detected %d deadlock(s)", deadlocks)
			}
		})
	}
}
