// Package audit describes the owner's administrative writes as audit_logs rows:
// the action and entity vocabulary, the row itself, and the diffs that fill its
// changes column.
//
// Everything here is pure — no database, no HTTP — so the rules that decide
// what an audit row says (which fields count as changed, how a Wi-Fi password
// is masked, how translations are spelled) can be tested on their own. The
// repository writes the rows (repository.InsertAuditLog) and the handlers
// decide when (handlers/audit.go).
package audit

import (
	"encoding/json"
	"reflect"
	"sort"

	"github.com/google/uuid"

	"karecik/backend/internal/models"
)

// The actions. Every value is listed in the audit_logs_action CHECK of
// migration 014, which turns a typo here into a failed write rather than a row
// the dashboard cannot label — and tests/audit_test.go writes each one to prove
// the two lists agree.
const (
	ActionProductCreate    = "product.create"
	ActionProductUpdate    = "product.update"
	ActionProductDelete    = "product.delete"
	ActionProductPrice     = "product.price"
	ActionProductBulkPrice = "product.bulk_price"
	ActionProductReorder   = "product.reorder"

	ActionCategoryCreate  = "category.create"
	ActionCategoryUpdate  = "category.update"
	ActionCategoryDelete  = "category.delete"
	ActionCategoryReorder = "category.reorder"

	ActionMenuCreate = "menu.create"
	ActionMenuUpdate = "menu.update"
	ActionMenuDelete = "menu.delete"

	ActionBusinessUpdate = "business.update"
	ActionPasswordChange = "account.password_change"
	ActionUploadCreate   = "upload.create"
)

// Actions lists every action, in the order the dashboard's filter offers them.
var Actions = []string{
	ActionProductCreate, ActionProductUpdate, ActionProductDelete,
	ActionProductPrice, ActionProductBulkPrice, ActionProductReorder,
	ActionCategoryCreate, ActionCategoryUpdate, ActionCategoryDelete, ActionCategoryReorder,
	ActionMenuCreate, ActionMenuUpdate, ActionMenuDelete,
	ActionBusinessUpdate, ActionPasswordChange, ActionUploadCreate,
}

// The entity types, likewise mirrored by a CHECK of migration 014.
const (
	EntityProduct  = "product"
	EntityCategory = "category"
	EntityMenu     = "menu"
	EntityBusiness = "business"
	EntityAccount  = "account"
	EntityUpload   = "upload"
)

// EntityTypes lists every entity type.
var EntityTypes = []string{
	EntityProduct, EntityCategory, EntityMenu, EntityBusiness, EntityAccount, EntityUpload,
}

// IsAction reports whether a string is one of Actions.
func IsAction(value string) bool { return contains(Actions, value) }

// IsEntityType reports whether a string is one of EntityTypes.
func IsEntityType(value string) bool { return contains(EntityTypes, value) }

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// MaskedSecret stands in for a secret value in the trail. Only the fact that it
// changed is recorded, never the value — a Wi-Fi password is readable by
// anyone at the table, but an audit log is read later, by whoever has the
// dashboard open, and has no business holding it.
const MaskedSecret = "••••"

// maskedKeys are the fields whose values the trail never stores.
var maskedKeys = map[string]bool{"wifi_password": true}

// Change is one field of a changes column: the value before and after. A
// created record has no "old" (null), a deleted one no "new".
type Change struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// Changes maps a field name to its change. Translations are spelled per
// language and field — "translations.en.name" — so a translation edit reads as
// exactly the text that moved.
type Changes map[string]Change

// Keys returns the changed field names in sorted order, for a stable test or
// log line.
func (c Changes) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Entry is one audit_logs row as a writer hands it over. BusinessID and the
// actor fields come from the session and the request (handlers.auditActor);
// the rest describes the write.
//
// IP and Port are the resolver's strings (clientip.Addr), "-" included: the
// repository turns "-" and "" into NULL, so a row never claims an address that
// was not established.
type Entry struct {
	BusinessID  uuid.UUID
	UserID      uuid.UUID // uuid.Nil is stored as NULL
	Action      string
	EntityType  string
	EntityID    string // "" is stored as NULL
	EntityLabel string
	Changes     Changes
	IP          string
	Port        string
	IPSource    string
}

// ------------------------------------------------------------------ diffing

// Fields no diff looks at: identity and bookkeeping that no request writes, the
// computed fields, and the values that move on their own. position is left to
// the reorder actions for products and categories, but it is a setting a menu
// save can write, so menus keep it.
var (
	productSkipped  = []string{"id", "created_at", "updated_at", "position"}
	categorySkipped = []string{"id", "created_at", "updated_at", "position", "product_count"}
	// price_updated_at is moved by a trigger, currency_symbol is derived from
	// currency (whose own change is recorded), and the other two are computed.
	menuSkipped     = []string{"id", "created_at", "updated_at", "category_count", "menu_url", "price_updated_at", "currency_symbol"}
	businessSkipped = []string{"id", "created_at", "updated_at", "home_url"}
)

// DiffProduct records what changed between two versions of a product. A nil
// before is a create — every field that carries a value, with a null "old" —
// and a nil after is a delete, the same the other way round.
func DiffProduct(before, after *models.Product) Changes {
	return diff(flatten(before, productSkipped), flatten(after, productSkipped), before == nil, after == nil)
}

// DiffCategory is DiffProduct for a category.
func DiffCategory(before, after *models.Category) Changes {
	return diff(flatten(before, categorySkipped), flatten(after, categorySkipped), before == nil, after == nil)
}

// DiffMenu records every setting that changed between two versions of a menu —
// the logo, the phone number, the links, the theme and colours, the languages,
// the VAT, price-date and Yerli Üretim switches and everything else a menu save
// can write — with the Wi-Fi password masked. A nil before is a create.
//
// It works from the menu's JSON form rather than from a hand-kept field list,
// so a setting added to models.Menu later is audited without anybody
// remembering to add it here.
func DiffMenu(before, after *models.Menu) Changes {
	return diff(flatten(before, menuSkipped), flatten(after, menuSkipped), before == nil, after == nil)
}

// DiffBusiness is DiffMenu for the account record: its name and slug.
func DiffBusiness(before, after *models.Business) Changes {
	return diff(flatten(before, businessSkipped), flatten(after, businessSkipped), before == nil, after == nil)
}

// Only keeps the listed fields of a set of changes — how a menu create records
// the settings its request actually named rather than every default the new
// row was given.
func (c Changes) Only(fields ...string) Changes {
	kept := make(Changes, len(fields))
	for _, field := range fields {
		if change, ok := c[field]; ok {
			kept[field] = change
		}
	}
	return kept
}

// flatten turns a record into a flat field map through its JSON form: the
// translations map is spelled out as "translations.<lang>.<field>" and every
// empty string reads as nil, so "" -> absent is never reported as a change.
// A nil record flattens to an empty map.
func flatten(record any, skipped []string) map[string]any {
	fields := make(map[string]any)
	if record == nil || reflect.ValueOf(record).IsNil() {
		return fields
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return fields
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return make(map[string]any)
	}
	for _, key := range skipped {
		delete(fields, key)
	}

	if translations, ok := fields["translations"].(map[string]any); ok {
		delete(fields, "translations")
		for lang, texts := range translations {
			if byField, ok := texts.(map[string]any); ok {
				for field, value := range byField {
					fields["translations."+lang+"."+field] = value
				}
			}
		}
	}

	for key, value := range fields {
		if text, ok := value.(string); ok && text == "" {
			fields[key] = nil
		}
	}
	return fields
}

// diff compares two flattened records. For a create (created) only the fields
// that carry something are kept, and for a delete (deleted) likewise the other
// way round: a record's false switches and empty lists are its defaults, not a
// change anybody made.
func diff(before, after map[string]any, created, deleted bool) Changes {
	changes := make(Changes)

	keys := make(map[string]bool, len(before)+len(after))
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}

	for key := range keys {
		was, is := before[key], after[key]
		if reflect.DeepEqual(was, is) {
			continue
		}
		if created && empty(is) {
			continue
		}
		if deleted && empty(was) {
			continue
		}
		if maskedKeys[key] {
			was, is = mask(was), mask(is)
		}
		changes[key] = Change{Old: was, New: is}
	}
	return changes
}

// empty reports a value that says nothing on its own: null, false, an empty
// list or object. Zero is NOT empty — a price of 0 is a price.
func empty(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case bool:
		return !v
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	return false
}

// mask replaces a secret with MaskedSecret, keeping null as null so that
// "set", "changed" and "removed" still read differently.
func mask(value any) any {
	if value == nil {
		return nil
	}
	return MaskedSecret
}
