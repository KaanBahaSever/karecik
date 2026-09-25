package svgsafe_test

// The SVG upload check, rule by rule: the logos real editors produce have to
// pass untouched, and every way of getting script, a fetch or embedded markup
// into an SVG has to be refused — including the spellings a pattern search on
// the raw bytes used to miss (entities, namespace prefixes, whitespace spliced
// into a scheme, an animation that writes the href at runtime).

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"karecik/backend/internal/svgsafe"
)

const pngDataURI = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

// clean are logos as design tools write them. Each has a viewBox and declares
// every namespace it uses, so each must come back byte for byte.
var clean = map[string]string{
	"minimal": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`,

	"prolog_comment_and_bom": "\xEF\xBB\xBF" + `<?xml version="1.0" encoding="UTF-8"?>
<!-- Generator: Adobe Illustrator 27.0.0 -->
<svg xmlns="http://www.w3.org/2000/svg" version="1.1" viewBox="0 0 200 80">
  <circle cx="40" cy="40" r="30" fill="#e11d48"/>
</svg>
`,

	"gradients_defs_and_use_by_fragment": `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 100 100">
  <defs>
    <linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#fff"/>
      <stop offset="1" stop-color="#000"/>
    </linearGradient>
    <path id="leaf" d="M0 0 L10 10 Z"/>
    <clipPath id="clip"><rect width="50" height="50"/></clipPath>
  </defs>
  <rect width="100" height="100" fill="url(#g)" clip-path="url('#clip')"/>
  <use href="#leaf" x="5"/>
  <use xlink:href="#leaf" x="20"/>
</svg>`,

	"inline_style_attribute_and_element": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 50 50">
  <style type="text/css"><![CDATA[
    .a { fill: #1d4ed8; stroke: url(#g); }
    @media (prefers-color-scheme: dark) { .a { fill: #93c5fd; } }
  ]]></style>
  <rect class="a" style="opacity:.9; fill: rgb(0, 0, 0)" width="50" height="50"/>
  <text x="5" y="20" font-family="Inter, sans-serif">Melly Coffee Co</text>
</svg>`,

	"embedded_raster_image": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><image href="` + pngDataURI + `" width="1" height="1"/></svg>`,

	"inkscape_metadata_namespaces": `<svg xmlns="http://www.w3.org/2000/svg"
  xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape"
  xmlns:sodipodi="http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd"
  xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
  xmlns:dc="http://purl.org/dc/elements/1.1/"
  viewBox="0 0 10 10">
  <sodipodi:namedview inkscape:zoom="1" inkscape:label="curl(ed) layer"/>
  <metadata><rdf:RDF><dc:title>Logo</dc:title></rdf:RDF></metadata>
  <g inkscape:groupmode="layer"><path d="M0 0h10v10z"/></g>
</svg>`,

	"harmless_animation": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><circle r="3"><animate attributeName="r" from="3" to="5" dur="1s"/></circle><set attributeName="fill" to="red"/></svg>`,

	"offset_and_opacity_are_not_event_attributes": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><stop offset="1" opacity="1"/></svg>`,

	// A prefix may be declared on the element that uses it or on any
	// ancestor; xml: is bound without a declaration.
	"prefixes_declared_where_they_are_used": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><g xmlns:x="urn:x" x:label="a"><x:meta/><use xmlns:xlink="http://www.w3.org/1999/xlink" xlink:href="#a"/></g><text xml:space="preserve">a</text></svg>`,
}

func TestCleanLogosAreAcceptedUnchanged(t *testing.T) {
	for name, logo := range clean {
		t.Run(name, func(t *testing.T) {
			out, err := svgsafe.Check([]byte(logo))
			if err != nil {
				t.Fatalf("a clean logo was refused: %v", err)
			}
			if !bytes.Equal(out, []byte(logo)) {
				t.Fatalf("a clean logo with a viewBox was rewritten:\n%s", out)
			}
		})
	}
}

// width and height without a viewBox: a rewrite of the root, so the logo
// scales inside an <img> instead of being cropped.
func TestAViewBoxIsAddedWhenOnlyTheSizeIsGiven(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"plain_numbers",
			`<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40"><rect width="120" height="40"/></svg>`,
			`<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40" viewBox="0 0 120 40"><rect width="120" height="40"/></svg>`},
		{"px_and_decimals",
			`<svg xmlns="http://www.w3.org/2000/svg" width="120.5px" height="40px" ><g/></svg>`,
			// The space the author left before ">" stays where it was; the new
			// attribute goes in front of it.
			`<svg xmlns="http://www.w3.org/2000/svg" width="120.5px" height="40px" viewBox="0 0 120.5 40" ><g/></svg>`},
		{"self_closing_root",
			`<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8"/>`,
			`<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8" viewBox="0 0 8 8"/>`},
		{"with_a_bom_and_prolog",
			"\xEF\xBB\xBF<?xml version=\"1.0\"?>\n<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"4\" height=\"2\">\n</svg>",
			"\xEF\xBB\xBF<?xml version=\"1.0\"?>\n<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"4\" height=\"2\" viewBox=\"0 0 4 2\">\n</svg>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := svgsafe.Check([]byte(tc.in))
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if string(out) != tc.want {
				t.Fatalf("got\n%s\nwant\n%s", out, tc.want)
			}
			// And the result is itself clean and stable.
			again, err := svgsafe.Check(out)
			if err != nil || !bytes.Equal(again, out) {
				t.Fatalf("the rewritten file does not pass unchanged: %v\n%s", err, again)
			}
		})
	}

	// A size in a physical unit or a percentage cannot become a viewBox, and
	// is left exactly as it was.
	for _, in := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="40"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="10mm" height="10mm"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="10"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="0" height="10"/>`,
	} {
		out, err := svgsafe.Check([]byte(in))
		if err != nil {
			t.Fatalf("%s: refused: %v", in, err)
		}
		if string(out) != in {
			t.Fatalf("%s was rewritten to %s", in, out)
		}
	}
}

// SVG copied out of an HTML page leans on the declarations HTML supplies
// implicitly: the root has no xmlns, and xlink:href is used undeclared. Served
// as a file of its own such an SVG draws nothing, so the root is given them.
func TestMissingNamespaceDeclarationsAreAdded(t *testing.T) {
	const (
		svgNS   = `xmlns="http://www.w3.org/2000/svg"`
		xlinkNS = `xmlns:xlink="http://www.w3.org/1999/xlink"`
	)
	for _, tc := range []struct {
		name, in, want string
	}{
		{"no_namespace",
			`<svg viewBox="0 0 10 10"><rect width="1" height="1"/></svg>`,
			`<svg viewBox="0 0 10 10" ` + svgNS + `><rect width="1" height="1"/></svg>`},
		{"no_namespace_and_no_viewbox",
			`<svg width="8" height="4"/>`,
			`<svg width="8" height="4" ` + svgNS + ` viewBox="0 0 8 4"/>`},
		{"undeclared_xlink",
			`<svg ` + svgNS + ` viewBox="0 0 10 10"><defs><rect id="a"/></defs><use xlink:href="#a"/></svg>`,
			`<svg ` + svgNS + ` viewBox="0 0 10 10" ` + xlinkNS + `><defs><rect id="a"/></defs><use xlink:href="#a"/></svg>`},
		{"xlink_declared_only_on_a_sibling",
			`<svg ` + svgNS + ` viewBox="0 0 10 10"><use ` + xlinkNS + ` xlink:href="#a"/><use xlink:href="#a"/></svg>`,
			`<svg ` + svgNS + ` viewBox="0 0 10 10" ` + xlinkNS + `><use ` + xlinkNS + ` xlink:href="#a"/><use xlink:href="#a"/></svg>`},
		{"copied_from_an_html_page",
			"\xEF\xBB\xBF<svg\n  width=\"24\" height=\"24\"\n>\n<symbol id=\"i\"><path d=\"M0 0h24v24z\"/></symbol>\n<use xlink:href=\"#i\"/>\n</svg>",
			"\xEF\xBB\xBF<svg\n  width=\"24\" height=\"24\" " + svgNS + " " + xlinkNS + " viewBox=\"0 0 24 24\"\n>\n<symbol id=\"i\"><path d=\"M0 0h24v24z\"/></symbol>\n<use xlink:href=\"#i\"/>\n</svg>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := svgsafe.Check([]byte(tc.in))
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if string(out) != tc.want {
				t.Fatalf("got\n%s\nwant\n%s", out, tc.want)
			}
			again, err := svgsafe.Check(out)
			if err != nil || !bytes.Equal(again, out) {
				t.Fatalf("the rewritten file does not pass unchanged: %v\n%s", err, again)
			}
		})
	}

	// The declarations change what a browser draws, never what the check
	// decides: a namespace-less file is refused for the same content as one
	// with its namespace.
	for _, in := range []string{
		`<svg><script>alert(1)</script></svg>`,
		`<svg><use xlink:href="https://evil.example/s.svg#a"/></svg>`,
		`<svg><a><set attributeName="xlink:href" to="#x"/></a></svg>`,
	} {
		if out, err := svgsafe.Check([]byte(in)); !errors.Is(err, svgsafe.ErrUnsafe) {
			t.Errorf("%s: accepted as\n%s", in, out)
		}
	}
}

// Every vector below must be refused, and refused as unsafe — not as an I/O
// error — so the handler answers 422.
func TestAttackVectorsAreRefused(t *testing.T) {
	const open = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 10 10">`
	const end = `</svg>`

	for name, svg := range map[string]string{
		// Elements that run or embed.
		"script":                      open + `<script>alert(1)</script>` + end,
		"script_uppercase":            open + `<SCRIPT>alert(1)</SCRIPT>` + end,
		"script_in_another_namespace": open + `<x:script xmlns:x="urn:x">alert(1)</x:script>` + end,
		"foreign_object":              open + `<foreignObject><div xmlns="http://www.w3.org/1999/xhtml">x</div></foreignObject>` + end,
		"xhtml_element":               open + `<h:img xmlns:h="http://www.w3.org/1999/xhtml" src="#x"/>` + end,
		"iframe":                      open + `<iframe src="#x"/>` + end,
		"embed":                       open + `<embed src="#x"/>` + end,
		"object":                      open + `<object data="#x"/>` + end,
		"handler":                     open + `<handler type="application/ecmascript">alert(1)</handler>` + end,
		"listener":                    open + `<listener event="click" handler="#h"/>` + end,

		// Event attributes, however spelled.
		"onload_on_the_root":     `<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`,
		"onclick_uppercase":      open + `<rect ONCLICK="alert(1)"/>` + end,
		"on_attribute_namespace": open + `<rect x:onmouseover="alert(1)" xmlns:x="urn:x"/>` + end,

		// References that leave the document.
		"external_href":           open + `<image href="https://tracker.example/p.png"/>` + end,
		"external_xlink_href":     open + `<use xlink:href="https://evil.example/sprite.svg#a"/>` + end,
		"relative_href":           open + `<use href="other.svg#a"/>` + end,
		"protocol_relative_href":  open + `<image href="//evil.example/x.png"/>` + end,
		"javascript_href":         open + `<a href="javascript:alert(1)"><rect/></a>` + end,
		"javascript_entity_href":  open + `<a href="&#106;avascript:alert(1)"><rect/></a>` + end,
		"javascript_tab_href":     open + "<a href=\"java\tscript:alert(1)\"><rect/></a>" + end,
		"vbscript_href":           open + `<a xlink:href="vbscript:msgbox(1)"><rect/></a>` + end,
		"svg_data_uri":            open + `<image href="data:image/svg+xml;base64,PHN2Zz48L3N2Zz4="/>` + end,
		"html_data_uri":           open + `<a href="data:text/html,&lt;script&gt;alert(1)&lt;/script&gt;"><rect/></a>` + end,
		"src_attribute":           open + `<image src="https://evil.example/x.png"/>` + end,
		"script_scheme_elsewhere": open + `<rect filter="javascript:alert(1)"/>` + end,
		"data_text_in_a_value":    open + `<rect mask="data:text/plain,x"/>` + end,
		"external_url_in_fill":    open + `<rect fill="url(https://evil.example/p.svg#g)"/>` + end,
		"unclosed_url_in_fill":    open + `<rect fill="url(https://evil.example/p.svg#g"/>` + end,
		"data_svg_url_in_fill":    open + `<rect fill="url(data:image/svg+xml,x)"/>` + end,

		// Animations that write a link or a handler.
		"animate_href":        open + `<a><animate attributeName="href" values="javascript:alert(1)"/></a>` + end,
		"set_xlink_href":      open + `<a><set attributeName="xlink:href" to="https://evil.example"/></a>` + end,
		"animate_to_onclick":  open + `<rect><set attributeName="onclick" to="x"/></rect>` + end,
		"animate_transform":   open + `<a><animateTransform attributeName="HREF" to="#x"/></a>` + end,
		"animate_href_benign": open + `<a><set attributeName="href" to="#x"/></a>` + end,

		// Stylesheets.
		"style_import":             open + `<style>@import url("https://evil.example/x.css");</style>` + end,
		"style_import_split":       open + `<style>@im<!-- -->port url(#x);</style>` + end,
		"style_external_url":       open + `<style>.a{background:url(https://evil.example/x.png)}</style>` + end,
		"style_javascript":         open + `<style>.a{background:url(javascript:alert(1))}</style>` + end,
		"style_expression":         open + `<style>.a{width:expression(alert(1))}</style>` + end,
		"style_escape":             open + `<style>.a{background:u\72l(https://evil.example)}</style>` + end,
		"style_behavior":           open + `<style>.a{behavior:url(#x)}</style>` + end,
		"style_moz_binding":        open + `<style>.a{-moz-binding:url(#x)}</style>` + end,
		"style_attribute_external": open + `<rect style="fill:url(https://evil.example/g)"/>` + end,
		"style_attribute_import":   open + `<rect style="@import 'x.css'"/>` + end,

		// Declarations, instructions and structure.
		"doctype":                  `<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd"><svg xmlns="http://www.w3.org/2000/svg"/>`,
		"entity_declaration":       `<!DOCTYPE svg [<!ENTITY x "javascript:alert(1)">]><svg xmlns="http://www.w3.org/2000/svg"><a href="&x;"/></svg>`,
		"billion_laughs":           `<!DOCTYPE lolz [<!ENTITY lol "lol"><!ENTITY lol2 "&lol;&lol;">]><svg xmlns="http://www.w3.org/2000/svg">&lol2;</svg>`,
		"undefined_entity":         open + `<text>&nbsp;</text>` + end,
		"xml_stylesheet":           `<?xml-stylesheet href="https://evil.example/x.css"?><svg xmlns="http://www.w3.org/2000/svg"/>`,
		"prolog_after_the_root":    `<svg xmlns="http://www.w3.org/2000/svg"></svg><?xml version="1.0"?>`,
		"html_root":                `<html><body><svg xmlns="http://www.w3.org/2000/svg"/></body></html>`,
		"svg_root_in_xhtml":        `<svg xmlns="http://www.w3.org/1999/xhtml"/>`,
		"svg_root_in_no_namespace": `<svg xmlns=""/>`,
		"svg_root_undeclared":      `<s:svg/>`,
		"text_before_the_root":     `hello<svg xmlns="http://www.w3.org/2000/svg"/>`,
		"a_second_root":            `<svg xmlns="http://www.w3.org/2000/svg"/><svg xmlns="http://www.w3.org/2000/svg"/>`,
		"not_xml":                  `<svg xmlns="http://www.w3.org/2000/svg"><rect></svg>`,
		"empty_file":               ``,
		"a_png_labelled_svg":       "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR",
		"latin1_encoding":          `<?xml version="1.0" encoding="ISO-8859-1"?><svg xmlns="http://www.w3.org/2000/svg"/>`,
		"deep_nesting":             `<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat("<g>", 300) + strings.Repeat("</g>", 300) + `</svg>`,
		"cdata_script_like_text":   open + `<style><![CDATA[ .a{background:url(https://x.example)} ]]></style>` + end,
		"style_in_foreign_ns":      open + `<x:style xmlns:x="urn:x">@import "evil.css";</x:style>` + end,
		"text_after_the_root":      `<svg xmlns="http://www.w3.org/2000/svg"/>trailing`,
		"iframe_in_xhtml_ns_upper": open + `<H:IFRAME xmlns:H="http://www.w3.org/1999/xhtml"/>` + end,

		// Undeclared prefixes: a browser draws nothing from such a file.
		"undeclared_attribute_prefix":  open + `<g sodipodi:docname="logo.svg"/>` + end,
		"undeclared_element_prefix":    open + `<x:rect/>` + end,
		"undeclared_xlink_element":     `<svg xmlns="http://www.w3.org/2000/svg"><xlink:use/></svg>`,
		"prefix_declared_on_a_sibling": open + `<g xmlns:x="urn:x"/><x:rect/>` + end,
	} {
		t.Run(name, func(t *testing.T) {
			// animate_href_benign included: an animation of href is refused
			// even towards a fragment, because the rule is about WHICH
			// attribute an animation may write, not the value — values can be
			// computed (by, from, additive) in ways the check cannot follow.
			out, err := svgsafe.Check([]byte(svg))
			if err == nil {
				t.Fatalf("accepted:\n%s", out)
			}
			if !errors.Is(err, svgsafe.ErrUnsafe) {
				t.Fatalf("refused, but not as ErrUnsafe: %v", err)
			}
			t.Logf("refused: %v", err)
		})
	}
}
