// Package svgsafe decides whether an uploaded SVG is a picture or a program.
//
// An SVG is markup, and markup can run script: a <script> element, an onload
// attribute, a javascript: link, an <animate> that rewrites an href into one,
// an <iframe> smuggled in through a foreign namespace. Inside an <img> tag a
// browser runs none of that — but an uploaded file also has a URL of its own,
// served from the tenant's own origin, and anybody can open that URL as a
// document. So the file is checked before it is stored, and the server adds a
// sandboxing Content-Security-Policy when it serves one (router.Setup) — two
// independent layers, so that neither has to be perfect.
//
// The check is a PARSER, not a pattern list. The previous version searched the
// raw bytes with three regular expressions, which an entity reference
// ("&#106;avascript:"), a namespace prefix (<x:script>) or an animation that
// writes the href at runtime walked straight past. Here the file is read with
// encoding/xml, whose tokenizer decodes entities and resolves namespaces
// before any rule looks at a name or a value, and every rule is an allowlist
// wherever an allowlist is possible:
//
//   - the document is well-formed XML whose root element is <svg>, in the SVG
//     namespace or in none (see below); before it only an <?xml?> declaration,
//     comments and whitespace may appear;
//   - every namespace prefix is declared (see below for xlink:) — a browser
//     draws nothing at all from a file that uses an undeclared one, so it is
//     refused here rather than stored as a logo that silently never shows;
//   - no DOCTYPE or other <!...> declaration at all — that is where entity
//     definitions (and the billion-laughs expansion) live — and no processing
//     instruction but the XML declaration (<?xml-stylesheet?> loads CSS);
//   - no script-capable or embedding element: script, foreignObject, iframe,
//     embed, object, handler, listener — in any namespace — nor any element of
//     the XHTML namespace, which is how HTML gets into an SVG;
//   - no on* attribute of any kind;
//   - href, xlink:href and src only as a same-document fragment ("#logo") or a
//     data: URI of a PNG, JPEG, GIF or WebP image — nothing external, nothing
//     that is itself markup;
//   - animate, set, animateTransform and animateMotion may not target href,
//     xlink:href or an event attribute;
//   - no attribute value and no stylesheet mentions javascript:, vbscript: or
//     data:text, even with whitespace or control characters spliced into the
//     scheme;
//   - CSS — the style attribute and <style> — may not @import, may not use
//     url() except for a fragment or an image data: URI, may not use the legacy
//     script hooks expression(), behavior or -moz-binding, and may not contain a
//     backslash, because CSS escapes are how the other rules would be spelled
//     around.
//
// A clean file is stored byte for byte as it was uploaded, except that the
// <svg> root is given what a browser needs to draw it as an image, and
// nothing else is ever rewritten:
//
//   - a root in no namespace is given xmlns="http://www.w3.org/2000/svg", and
//     an xlink: prefix used without a declaration gets
//     xmlns:xlink="http://www.w3.org/1999/xlink". An HTML page supplies both
//     implicitly, so SVG copied out of one usually has neither — and served
//     as an image/svg+xml file of its own, a root outside the SVG namespace
//     draws nothing. Every rule but the XHTML one reads a name by its local
//     part alone, so declaring these two namespaces changes nothing the
//     check decided;
//   - a root that has a numeric width and height but no viewBox is given
//     viewBox="0 0 width height". Without one the drawing does not scale
//     inside an <img> of another size — it is cropped instead — which is the
//     single most common reason a logo "looks broken" in the menu header.
package svgsafe

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Namespaces the rules name. xmlNamespace is what encoding/xml resolves the
// reserved xml: prefix to; it is bound in every document without a
// declaration.
const (
	svgNamespace   = "http://www.w3.org/2000/svg"
	xlinkNamespace = "http://www.w3.org/1999/xlink"
	xhtmlNamespace = "http://www.w3.org/1999/xhtml"
	xmlNamespace   = "http://www.w3.org/XML/1998/namespace"
)

// maxDepth bounds element nesting. A logo is a handful of levels deep; the cap
// only keeps a pathological file from costing more than a real one.
const maxDepth = 256

// ErrUnsafe wraps every refusal, so a caller can tell "this file is not
// acceptable" from an I/O failure with errors.Is. The wrapping error names the
// rule, which is logged and never shown to the uploader.
var ErrUnsafe = errors.New("svg refused")

func refuse(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrUnsafe, fmt.Sprintf(format, args...))
}

// forbiddenElements are refused by local name in every namespace, so a prefix
// bound to some other URI (<x:script xmlns:x="...">) changes nothing.
var forbiddenElements = map[string]bool{
	"script":        true,
	"foreignobject": true,
	"iframe":        true,
	"embed":         true,
	"object":        true,
	"handler":       true,
	"listener":      true,
}

// animationElements can rewrite another attribute while the image plays, so
// what they target is checked too.
var animationElements = map[string]bool{
	"animate":          true,
	"set":              true,
	"animatetransform": true,
	"animatemotion":    true,
}

// referenceAttributes load or link to another resource.
var referenceAttributes = map[string]bool{"href": true, "src": true}

// imageDataURI is the one kind of data: URI a reference or a CSS url() may
// hold: a raster image, base64 or not. SVG is deliberately absent — an SVG in
// a data: URI is markup again, and would need this whole check of its own.
var imageDataURI = regexp.MustCompile(`^data:image/(png|jpeg|jpg|gif|webp)(;[a-z0-9=._-]+)*(;base64)?,`)

// Check reads an uploaded SVG and returns the bytes to store — the upload
// itself, or the upload with a namespace declaration or a viewBox added to its
// root (see the package comment) — or an error wrapping ErrUnsafe that names
// the first rule it breaks.
func Check(data []byte) ([]byte, error) {
	// A UTF-8 byte order mark is common from Windows editors and is not
	// character data before the root; encoding/xml would read it as such.
	offset := 0
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		offset = 3
	}

	decoder := xml.NewDecoder(bytes.NewReader(data[offset:]))
	decoder.Strict = true
	// No entity map beyond the five XML predefines: an undefined &name; is an
	// error, and with DOCTYPE refused there is nowhere to define one.
	decoder.Entity = nil
	// A charset other than UTF-8 would need a CharsetReader; leaving it unset
	// refuses such a file, which a browser would read differently from us.
	decoder.CharsetReader = nil

	var (
		rootSeen   bool
		rootClosed bool
		depth      int
		inStyle    int // > 0 while inside a <style> element
		styleText  strings.Builder
		rootEnd    int // byte offset at which an attribute can join the root's start tag
		// What the root is missing (see the package comment).
		addNamespace bool
		addXlink     bool
		viewBox      string
		// The namespace URIs declared by each open element, and how many of
		// the open elements declare each one: a prefixed name whose Space is
		// not among them was never bound.
		declared [][]string
		inScope  = map[string]int{}
	)

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, refuse("not well-formed XML: %v", err)
		}

		switch t := token.(type) {
		case xml.ProcInst:
			// Only the XML declaration, and only before the root.
			if !strings.EqualFold(t.Target, "xml") || rootSeen {
				return nil, refuse("processing instruction <?%s?>", t.Target)
			}

		case xml.Directive:
			// <!DOCTYPE ...>, <!ENTITY ...> and every other declaration.
			return nil, refuse("declaration <!%s>", firstWord(string(t)))

		case xml.Comment:
			// Harmless anywhere.

		case xml.CharData:
			if !rootSeen || rootClosed {
				if len(bytes.TrimSpace(t)) > 0 {
					return nil, refuse("text outside the <svg> root")
				}
				continue
			}
			if inStyle > 0 {
				styleText.Write(t)
			}

		case xml.StartElement:
			if rootClosed {
				return nil, refuse("a second root element <%s>", t.Name.Local)
			}
			depth++
			if depth > maxDepth {
				return nil, refuse("nested deeper than %d elements", maxDepth)
			}
			// An element's own declarations are in scope for its name and
			// its attributes.
			uris := declarations(t.Attr)
			for _, uri := range uris {
				inScope[uri]++
			}
			declared = append(declared, uris)

			local := strings.ToLower(t.Name.Local)
			if !rootSeen {
				rootSeen = true
				if local != "svg" || (t.Name.Space != svgNamespace && t.Name.Space != "") {
					return nil, refuse("the root element is <%s> in namespace %q, not an SVG <svg>",
						t.Name.Local, t.Name.Space)
				}
				if t.Name.Space == "" {
					// A root that is in no namespace because nothing declares
					// one is given the SVG namespace. xmlns="" asks for no
					// namespace outright; no browser draws that as an SVG,
					// and adding a second xmlns would not be XML.
					if hasDefaultDeclaration(t.Attr) {
						return nil, refuse(`the root <svg> declares xmlns=""`)
					}
					addNamespace = true
				}
				// InputOffset is just past this start tag's ">" (or "/>").
				rootEnd = tagInsertionPoint(data, int(decoder.InputOffset())+offset)
				if box, ok := missingViewBox(t.Attr); ok {
					viewBox = box
				}
			}

			if forbiddenElements[local] {
				return nil, refuse("element <%s>", t.Name.Local)
			}
			if t.Name.Space == xhtmlNamespace {
				return nil, refuse("an XHTML element <%s>", t.Name.Local)
			}
			if err := checkAttributes(local, t.Attr); err != nil {
				return nil, err
			}
			usesXlink, err := checkPrefixes(t, inScope)
			if err != nil {
				return nil, err
			}
			addXlink = addXlink || usesXlink
			if local == "style" {
				inStyle++
			}

		case xml.EndElement:
			depth--
			// The strict decoder pairs every end tag with its start tag, so
			// this closes the element whose declarations are on top.
			for _, uri := range declared[len(declared)-1] {
				inScope[uri]--
			}
			declared = declared[:len(declared)-1]
			if strings.EqualFold(t.Name.Local, "style") && inStyle > 0 {
				inStyle--
				if inStyle == 0 {
					if err := checkCSS(styleText.String()); err != nil {
						return nil, err
					}
					styleText.Reset()
				}
			}
			if depth == 0 {
				rootClosed = true
			}
		}
	}

	if !rootSeen {
		return nil, refuse("no <svg> root element")
	}

	var added strings.Builder
	if addNamespace {
		added.WriteString(` xmlns="` + svgNamespace + `"`)
	}
	if addXlink {
		added.WriteString(` xmlns:xlink="` + xlinkNamespace + `"`)
	}
	if viewBox != "" {
		added.WriteString(` viewBox="` + viewBox + `"`)
	}
	if added.Len() == 0 {
		return data, nil
	}
	fixed := make([]byte, 0, len(data)+added.Len())
	fixed = append(fixed, data[:rootEnd]...)
	fixed = append(fixed, added.String()...)
	fixed = append(fixed, data[rootEnd:]...)
	return fixed, nil
}

// declarations returns the namespace URIs an element's xmlns and xmlns:prefix
// attributes bind.
func declarations(attrs []xml.Attr) []string {
	var uris []string
	for _, attr := range attrs {
		if attr.Name.Space == "xmlns" || (attr.Name.Space == "" && attr.Name.Local == "xmlns") {
			uris = append(uris, attr.Value)
		}
	}
	return uris
}

// hasDefaultDeclaration reports an xmlns attribute among an element's own.
func hasDefaultDeclaration(attrs []xml.Attr) bool {
	for _, attr := range attrs {
		if attr.Name.Space == "" && attr.Name.Local == "xmlns" {
			return true
		}
	}
	return false
}

// checkPrefixes refuses an element whose name or attributes use a namespace
// prefix nothing in scope declares: encoding/xml leaves such a name's Space as
// the bare prefix rather than a URI, and a browser refuses to draw the file.
// xlink: is reported instead of refused, because it is the one prefix an HTML
// page binds implicitly — SVG copied out of one uses it undeclared, and the
// declaration can simply be added to the root.
func checkPrefixes(t xml.StartElement, inScope map[string]int) (usesXlink bool, err error) {
	bound := func(space string) bool {
		return space == "" || space == xmlNamespace || inScope[space] > 0
	}
	if !bound(t.Name.Space) {
		return false, refuse("the undeclared prefix %s: on <%s>", t.Name.Space, t.Name.Local)
	}
	for _, attr := range t.Attr {
		switch {
		case attr.Name.Space == "xmlns" || bound(attr.Name.Space):
		case attr.Name.Space == "xlink":
			usesXlink = true
		default:
			return false, refuse("the undeclared prefix %s: on %s of <%s>",
				attr.Name.Space, attr.Name.Local, t.Name.Local)
		}
	}
	return usesXlink, nil
}

// checkAttributes applies the attribute rules to one element.
func checkAttributes(element string, attrs []xml.Attr) error {
	for _, attr := range attrs {
		name := strings.ToLower(attr.Name.Local)
		value := attr.Value

		// xmlns declarations bind prefixes; their values are namespace URIs,
		// which are identifiers and never fetched.
		if attr.Name.Space == "xmlns" || (attr.Name.Space == "" && name == "xmlns") {
			continue
		}

		if strings.HasPrefix(name, "on") {
			return refuse("event attribute %s on <%s>", attr.Name.Local, element)
		}
		if referenceAttributes[name] && !safeReference(value) {
			return refuse("%s=%q on <%s> is not a #fragment or an image data: URI",
				attr.Name.Local, truncate(value), element)
		}
		if animationElements[element] && name == "attributename" {
			target := strings.ToLower(strings.TrimSpace(value))
			if cut := strings.LastIndex(target, ":"); cut >= 0 {
				target = target[cut+1:]
			}
			if referenceAttributes[target] || strings.HasPrefix(target, "on") {
				return refuse("<%s> animates %s", element, value)
			}
		}
		if hasScriptScheme(value) {
			return refuse("a script URL in %s on <%s>", attr.Name.Local, element)
		}
		if name == "style" {
			if err := checkCSS(value); err != nil {
				return err
			}
		} else if err := checkURLFunctions(value); err != nil {
			// fill="url(#gradient)" is how every gradient is referenced; an
			// external url() in a presentation attribute is a fetch like any
			// other.
			return err
		}
	}
	return nil
}

// safeReference reports whether an href or src stays inside the document or
// holds an image inline.
func safeReference(value string) bool {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "#") {
		return true
	}
	return imageDataURI.MatchString(strings.ToLower(value))
}

// hasScriptScheme reports a javascript:, vbscript: or data:text scheme anywhere
// in a value. Whitespace and control characters are removed first, because a
// browser strips them from a URL before it reads the scheme — "java\tscript:"
// is javascript: to it.
func hasScriptScheme(value string) bool {
	compact := strings.Map(func(r rune) rune {
		if r <= ' ' || r == 0x7f {
			return -1
		}
		return r
	}, strings.ToLower(value))
	return strings.Contains(compact, "javascript:") ||
		strings.Contains(compact, "vbscript:") ||
		strings.Contains(compact, "data:text")
}

// cssURL finds the argument of every url(...) in a stylesheet or attribute,
// and cssURLOpen every place one starts. The word boundary keeps a word
// that merely ends in "url" — an editor's label "curl(ed)" — from counting
// as one.
var (
	cssURL     = regexp.MustCompile(`(?i)\burl\s*\(\s*(['"]?)([^'")]*)(['"]?)\s*\)`)
	cssURLOpen = regexp.MustCompile(`(?i)\burl\s*\(`)
)

// checkCSS applies the stylesheet rules to a <style> body or a style
// attribute.
func checkCSS(css string) error {
	lower := strings.ToLower(css)
	if strings.Contains(css, `\`) {
		return refuse("a CSS escape in a stylesheet")
	}
	if hasScriptScheme(css) {
		return refuse("a script URL in a stylesheet")
	}
	for _, hook := range []string{"@import", "expression(", "behavior", "-moz-binding"} {
		if strings.Contains(lower, hook) {
			return refuse("%s in a stylesheet", hook)
		}
	}
	return checkURLFunctions(css)
}

// checkURLFunctions refuses a url() that is neither a fragment nor an image
// data: URI. "url(" with no closing parenthesis cannot be matched and is
// refused as well rather than guessed at.
func checkURLFunctions(value string) error {
	opened := len(cssURLOpen.FindAllStringIndex(value, -1))
	if opened == 0 {
		return nil
	}
	matches := cssURL.FindAllStringSubmatch(value, -1)
	if len(matches) != opened {
		return refuse("a url() that cannot be read")
	}
	for _, match := range matches {
		if !safeReference(match[2]) {
			return refuse("url(%s) is not a #fragment or an image data: URI", truncate(match[2]))
		}
	}
	return nil
}

// missingViewBox reports the viewBox a root needs: "0 0 width height" when it
// has width and height as plain numbers (or px) and no viewBox, and false
// otherwise — including for a percentage or a physical unit, whose user-space
// size cannot be read off the attribute.
func missingViewBox(attrs []xml.Attr) (string, bool) {
	var width, height string
	for _, attr := range attrs {
		if attr.Name.Space != "" {
			continue
		}
		switch attr.Name.Local {
		case "viewBox", "viewbox":
			return "", false
		case "width":
			width = attr.Value
		case "height":
			height = attr.Value
		}
	}
	w, okW := userUnits(width)
	h, okH := userUnits(height)
	if !okW || !okH {
		return "", false
	}
	return "0 0 " + w + " " + h, true
}

// userUnits reads a length that is a positive plain number or a px value and
// returns it formatted for a viewBox.
func userUnits(length string) (string, bool) {
	length = strings.TrimSuffix(strings.TrimSpace(length), "px")
	value, err := strconv.ParseFloat(strings.TrimSpace(length), 64)
	if err != nil || value <= 0 || value > 1e6 {
		return "", false
	}
	return strconv.FormatFloat(value, 'f', -1, 64), true
}

// tagInsertionPoint returns where an attribute can be added to a start tag
// that ends just before end: before its "/>" or its ">", after any whitespace
// in front of them, so the new attribute sits where one typed by hand would.
func tagInsertionPoint(data []byte, end int) int {
	at := end - 1 // the ">"
	if at > 0 && data[at-1] == '/' {
		at--
	}
	for at > 0 && (data[at-1] == ' ' || data[at-1] == '\t' || data[at-1] == '\n' || data[at-1] == '\r') {
		at--
	}
	return at
}

func firstWord(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// truncate keeps a quoted value short enough for a log line.
func truncate(s string) string {
	const limit = 60
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…"
}
