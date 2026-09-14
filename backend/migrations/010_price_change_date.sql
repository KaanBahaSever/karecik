-- Karecik — the "prices valid from" date follows every price change
--
-- The customer menu footer prints "Fiyatlarımız dd.mm.yyyy tarihinden itibaren
-- geçerlidir." from menus.price_updated_at. Before this migration exactly one
-- code path moved that date: the bulk price endpoint, which set it on every
-- apply — even one that changed no price at all — while a price edited in the
-- product dialog or through the inline quick edit left it where it was. The
-- footer could therefore print a date older than the prices above it, or a
-- newer one for prices nobody had touched.
--
-- WHY A TRIGGER AND NOT A LINE IN EACH HANDLER
--
-- The price of an existing product is changed through three endpoints today:
-- PUT /api/products/:id, PATCH /api/products/:id/price and
-- POST /api/products/bulk-price. A date each of them has to remember to move is
-- a date the next one forgets, and "did a price actually change?" is a question
-- all of them would have to answer identically. The trigger sits on the
-- products rows themselves: whatever changes a price moves the date, whatever
-- does not change one cannot, and the handlers carry no timestamp code at all.
-- Anyone about to add a second mechanism on the Go side should read this first.
--
-- WHAT COUNTS AS A PRICE CHANGE
--
-- An UPDATE of a products row where at least one of these differs (IS DISTINCT
-- FROM) between OLD and NEW:
--
--   - price
--   - compare_price
--   - the ordered list of option surcharges,
--     jsonb_path_query_array(options, '$[*].items[*].price')
--
-- NOT a price change: creating or deleting a product, reordering, moving a
-- product without changing its price, toggling is_active or is_featured, or
-- editing translations, allergens, badges, the image, calories or the name of
-- an option. The product dialog sends every field on every save, so the test
-- has to be on values and not on which columns an UPDATE happened to name.
--
-- The surcharges are compared as jsonb and never as text. jsonb compares
-- numbers numerically, so 10 and 10.0 are the same surcharge and re-saving
-- identical options does not move the date.
--
-- WHICH MENU
--
-- The menu of the product's NEW category: NEW.category_id -> categories.menu_id.
-- A product moved into another menu with a new price dates the menu that now
-- prints that price; the menu it left keeps its date.
--
-- Additive and idempotent: the function is CREATE OR REPLACE and the trigger
-- uses the same DROP TRIGGER IF EXISTS + CREATE TRIGGER pair as 001 and 003,
-- so re-running the file is harmless. No column is added and no row is written.
--
-- The reverse script lives in migrations/down/ and is NEVER executed
-- automatically — see the note at the top of that file.

-- ------------------------------------------------------------- function
-- Moves the price date of the menu the product now sits on. The menu's own
-- updated_at moves with it, through menus_set_updated_at from 003.
--
-- The business_id predicate is belt-and-braces. The handlers already prove a
-- category belongs to the business before they put a product into it, but a
-- write that trusts an id alone is one careless future caller away from
-- touching a foreign tenant's row, so the rule every repository write follows
-- holds here too.
--
-- The IS DISTINCT FROM now() guard is what keeps a bulk update cheap. now() is
-- the start time of the current transaction and stays constant inside it, so
-- when one statement changes N prices of the same menu the first call writes
-- the menu row and the remaining N-1 find it already at now() and match
-- nothing: the menu row is written once, not N times.
--
-- RETURN NULL because the return value of an AFTER trigger is ignored.
CREATE OR REPLACE FUNCTION karecik_touch_menu_price_date()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE menus m
       SET price_updated_at = now()
      FROM categories c
     WHERE c.id = NEW.category_id
       AND m.id = c.menu_id
       AND m.business_id = NEW.business_id
       AND m.price_updated_at IS DISTINCT FROM now();
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- -------------------------------------------------------------- trigger
-- UPDATE OF names the three columns, so an UPDATE whose SET list names none of
-- them — a reorder, the is_active toggle in the menu editor — never fires it.
-- The WHEN clause then decides on values, which is what the product dialog's
-- full payload needs.
--
-- AFTER, not BEFORE: the queued row events of an AFTER trigger fire at the end
-- of the statement, so a statement takes every product row lock it needs before
-- the function takes the menu row lock. repository.ApplyPrices explains why
-- that order matters.
DROP TRIGGER IF EXISTS products_touch_menu_price_date ON products;
CREATE TRIGGER products_touch_menu_price_date
    AFTER UPDATE OF price, compare_price, options ON products
    FOR EACH ROW
    WHEN (
        OLD.price IS DISTINCT FROM NEW.price
        OR OLD.compare_price IS DISTINCT FROM NEW.compare_price
        OR jsonb_path_query_array(OLD.options, '$[*].items[*].price')
           IS DISTINCT FROM jsonb_path_query_array(NEW.options, '$[*].items[*].price')
    )
    EXECUTE FUNCTION karecik_touch_menu_price_date();
