-- Karecik — where the customer menu draws its contact block, and the owner's
-- own links inside it
--
-- The contact details of a menu — the phone number, Instagram, the Wi-Fi name
-- and password — have had fixed places on the customer menu: a Wi-Fi card on
-- the home view and a compact list in the footer of the product screens. This
-- migration adds the two settings that hand that block to the owner:
--
--   contact_display  where the block is drawn — four modes, below
--   links            the owner's own entries, drawn after the fixed ones
--
-- ADDITIVE ONLY: two columns on menus and one CHECK constraint on each, and
-- nothing else. No table is dropped, no existing column or constraint is
-- changed and no stored value is rewritten; every existing menu simply receives
-- the two column defaults.
--
-- New columns need a new file: database/migrate.go skips every file already
-- recorded in schema_migrations, so a column added to 010 would never reach a
-- database that has run it, and every menu read there would then fail against
-- a column that is not there.
--
-- Idempotent: both ADD COLUMNs are IF NOT EXISTS guarded, and each CHECK
-- constraint is dropped IF EXISTS right before it is added — the pair 004
-- uses — because a CHECK constraint has no IF NOT EXISTS form. Dropping first,
-- rather than looking the name up in pg_constraint as 006 and 007 do, also
-- replaces a same-named constraint whose definition has drifted, so a re-run
-- always leaves exactly the definitions below. Re-running the whole file is
-- harmless.
--
-- The reverse script lives in migrations/down/ and is NEVER executed
-- automatically — see the note at the top of that file.

-- ---------------------------------------------------------------- menus
-- contact_display: where the customer menu draws the contact block — the
--                  Wi-Fi, Instagram and phone entries, then the menu's links.
--
--                    'inline'  "Yan yana"          one row of chips on the
--                                                  home view; a chip opens
--                                                  its details
--                    'list'    "Açık liste"        the same entries as an
--                                                  always-open list on the
--                                                  home view
--                    'footer'  "Sadece alt bilgi"  nothing on the home view;
--                                                  the entries only in the
--                                                  footer of the product
--                                                  screens
--                    'hidden'  "Hiç gösterme"      the entries nowhere
--
--                  inline, list and footer all keep the compact list in the
--                  footer of the product screens; hidden drops that too. In
--                  hidden mode the public payload does not even carry the
--                  entries: the API sends phone, instagram, wifi_ssid and
--                  wifi_password as null and links as [], so hiding them is
--                  not merely cosmetic.
--
--                  NOT NULL DEFAULT 'inline', so scanMenu reads a plain string
--                  and every existing menu starts in the mode the API contract
--                  names as the default.
ALTER TABLE menus ADD COLUMN IF NOT EXISTS contact_display TEXT NOT NULL DEFAULT 'inline';

-- links: the owner's own entries of the contact block, in the owner's order —
--        [{"id": "...", "label": "Rezervasyon", "url": "https://..."}].
--
--        Like products.badges and products.options, the element shape is
--        validated in Go and not by the database. utils.CheckMenuLink holds
--        the rules of one entry; handlers/menu.go (SanitizeMenuLinks) refuses
--        a list that breaks one of them — or holds more than 8 entries — with
--        a Turkish 422 before an UPDATE is ever built, and repository/menu.go
--        checks every stored entry again before it lets it into the customer
--        payload. The rules of one entry:
--
--          label  trimmed of Unicode White_Space; no C0 or C1 control
--                 character; something visible left once the invisible
--                 format characters are removed and the rest is trimmed
--                 again; at most 40 characters (runes)
--          url    trimmed of Unicode White_Space; at most 500 bytes; starts
--                 with http:// or https:// in any letter case; no
--                 whitespace, control character, invisible format character
--                 or any of the characters < > " ` { } | \ ^ [ ] anywhere;
--                 parsed by Go's net/url with no userinfo; a host name of 1
--                 to 253 characters in at least two dot-separated labels of
--                 1 to 63 letters, ASCII digits or hyphens, none starting or
--                 ending with a hyphen and the last one holding a letter; a
--                 port, when there is a colon after the host name, of 1 to 5
--                 digits valued 1 to 65535
--
--        docs/API.md spells the rules out in full.
--
--        NOT NULL DEFAULT '[]' rather than nullable, so scanMenu never meets a
--        NULL, the payload always carries an array, and every existing menu
--        starts with no links.
ALTER TABLE menus ADD COLUMN IF NOT EXISTS links JSONB NOT NULL DEFAULT '[]'::jsonb;

-- ---------------------------------------------------------- constraints
-- menus_contact_display_check keeps the column to the four modes above. The
-- ids are exactly utils.ContactDisplayModes, which handlers/menu.go checks
-- with utils.IsValidContactDisplay first, so a value the API accepts always
-- satisfies the constraint and a bad one earns a 422 rather than a 500.
ALTER TABLE menus DROP CONSTRAINT IF EXISTS menus_contact_display_check;
ALTER TABLE menus
    ADD CONSTRAINT menus_contact_display_check
    CHECK (contact_display IN ('inline', 'list', 'footer', 'hidden'));

-- menus_links_check keeps links an array. NOT NULL refuses SQL NULL but not
-- the jsonb value null, and neither refuses an object or a string, while the
-- API contract promises every reader a list. The element shape stays in Go,
-- for the reason given above.
ALTER TABLE menus DROP CONSTRAINT IF EXISTS menus_links_check;
ALTER TABLE menus
    ADD CONSTRAINT menus_links_check
    CHECK (jsonb_typeof(links) = 'array');
