package models_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"testing"

	"karecik/backend/internal/models"
)

// Every menu read scans menus.links into a MenuLinks, so its decoding is what
// stands between one malformed stored element and a 500 on every menu read.
func TestMenuLinksUnmarshalJSONNeverFails(t *testing.T) {
	valid := models.MenuLink{ID: "rez", Label: "Rezervasyon", URL: "https://rezervasyon.example"}

	cases := []struct {
		name string
		data string
		want models.MenuLinks
	}{
		{"empty_array", `[]`, models.MenuLinks{}},
		{"valid_entry", `[{"id":"rez","label":"Rezervasyon","url":"https://rezervasyon.example"}]`, models.MenuLinks{valid}},
		{"spaces_as_jsonb_prints_them", `[{"id": "rez", "url": "https://rezervasyon.example", "label": "Rezervasyon"}]`, models.MenuLinks{valid}},

		{"null", `null`, models.MenuLinks{}},
		{"string", `"x"`, models.MenuLinks{}},
		{"number", `5`, models.MenuLinks{}},
		{"object", `{"id":"rez","label":"Rezervasyon","url":"https://rezervasyon.example"}`, models.MenuLinks{}},
		{"bool", `true`, models.MenuLinks{}},

		{"null_element", `[null]`, models.MenuLinks{}},
		{"string_element", `["x"]`, models.MenuLinks{}},
		{"number_elements", `[1,2]`, models.MenuLinks{}},
		{"array_element", `[[1,2]]`, models.MenuLinks{}},
		{"numeric_label", `[{"label":42,"url":"https://a.example"}]`, models.MenuLinks{}},
		{"object_url", `[{"label":"a","url":{}}]`, models.MenuLinks{}},
		{"missing_label", `[{"url":"https://a.example"}]`, models.MenuLinks{}},
		{"missing_url", `[{"label":"a"}]`, models.MenuLinks{}},
		{"null_label", `[{"label":null,"url":"https://a.example"}]`, models.MenuLinks{}},
		{"capitalised_members_are_not_the_members", `[{"Label":"a","URL":"https://a.example"}]`, models.MenuLinks{}},

		{"non_string_id_reads_as_empty", `[{"id":5,"label":"a","url":"https://a.example"}]`,
			models.MenuLinks{{ID: "", Label: "a", URL: "https://a.example"}}},
		{"missing_id_reads_as_empty", `[{"label":"a","url":"https://a.example"}]`,
			models.MenuLinks{{ID: "", Label: "a", URL: "https://a.example"}}},
		{"strings_are_taken_as_they_are", `[{"id":" x ","label":"","url":"not a url"}]`,
			models.MenuLinks{{ID: " x ", Label: "", URL: "not a url"}}},
		{"extra_members_are_ignored", `[{"id":"rez","label":"Rezervasyon","url":"https://rezervasyon.example","onclick":"alert(1)"}]`,
			models.MenuLinks{valid}},

		{"good_entries_survive_their_bad_neighbours",
			`[{"label":42}, "x", [1,2], {"url":{}}, null, {"id":"rez","label":"Rezervasyon","url":"https://rezervasyon.example"}, 7]`,
			models.MenuLinks{valid}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Pre-filled, so a decoder that leaves the old value behind on a bad
			// input is caught as well.
			links := models.MenuLinks{{ID: "stale", Label: "stale", URL: "stale"}}
			if err := json.Unmarshal([]byte(tc.data), &links); err != nil {
				t.Fatalf("json.Unmarshal(%s) returned %v, want no error at all", tc.data, err)
			}
			if links == nil {
				t.Fatalf("json.Unmarshal(%s) left a nil list, want %v", tc.data, tc.want)
			}
			if fmt.Sprint(links) != fmt.Sprint(tc.want) || len(links) != len(tc.want) {
				t.Fatalf("json.Unmarshal(%s) = %+v, want %+v", tc.data, links, tc.want)
			}
		})
	}
}

// Every menu read scans menus.links through Scan, so it must never fail —
// JSON nested more than 10000 levels deep included, which json.Unmarshal
// refuses before any UnmarshalJSON runs. A value it cannot read as an array
// writes one line to the standard logger; one it can read writes none.
func TestMenuLinksScanNeverFails(t *testing.T) {
	var buf bytes.Buffer
	savedWriter, savedFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(savedWriter)
		log.SetFlags(savedFlags)
	}()

	valid := models.MenuLink{ID: "rez", Label: "Rezervasyon", URL: "https://rezervasyon.example"}
	good := `{"id":"rez","label":"Rezervasyon","url":"https://rezervasyon.example"}`
	nested := func(levels int) string { return strings.Repeat("[", levels) + strings.Repeat("]", levels) }

	cases := []struct {
		name  string
		src   any
		want  models.MenuLinks
		lines int
	}{
		{"text", "[" + good + "]", models.MenuLinks{valid}, 0},
		{"bytes", []byte("[" + good + "]"), models.MenuLinks{valid}, 0},
		{"sql_null", nil, models.MenuLinks{}, 0},
		{"json_null", "null", models.MenuLinks{}, 0},
		{"a_skipped_element_is_not_a_line", "[5," + good + "]", models.MenuLinks{valid}, 0},
		{"10001_levels", "[" + nested(10000) + "]", models.MenuLinks{}, 1},
		{"a_good_entry_next_to_an_element_10001_levels_deep", "[" + good + "," + nested(10000) + "]", models.MenuLinks{}, 1},
		{"an_element_exactly_10000_levels_deep", "[" + good + "," + nested(9999) + "]", models.MenuLinks{valid}, 0},
		{"invalid_json", "[{", models.MenuLinks{}, 1},
		{"an_object", good, models.MenuLinks{}, 1},
		{"an_unexpected_type", 42, models.MenuLinks{}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()
			links := models.MenuLinks{{ID: "stale", Label: "stale", URL: "stale"}}
			if err := links.Scan(tc.src); err != nil {
				t.Fatalf("Scan returned %v, want no error at all", err)
			}
			if links == nil || fmt.Sprint(links) != fmt.Sprint(tc.want) || len(links) != len(tc.want) {
				t.Fatalf("Scan read %#v, want %+v", links, tc.want)
			}
			var logged []string
			if buf.Len() > 0 {
				logged = strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
			}
			if len(logged) != tc.lines {
				t.Fatalf("Scan logged %d line(s), want %d: %q", len(logged), tc.lines, logged)
			}
			for _, line := range logged {
				if strings.Contains(line, "[[[") || strings.Contains(line, "Rezervasyon") {
					t.Errorf("the log line repeats the stored value: %.200s", line)
				}
			}
		})
	}
}

// The request path decodes into []MenuLink, and it depends on that staying
// strict: a label that is a number is a refusal there, not a skipped entry.
func TestMenuLinkDecodingStaysStrict(t *testing.T) {
	var links []models.MenuLink
	if err := json.Unmarshal([]byte(`[{"label":42,"url":"https://a.example"}]`), &links); err == nil {
		t.Fatalf("decoding a numeric label into []MenuLink succeeded with %+v, want an error", links)
	}
}
