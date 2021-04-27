package hy

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const filterFailsafe = "ZgotmplZ"

func attrEscaper(s string) string {
	return htmlReplacer(s, htmlReplacementTable)
}

var htmlReplacementTable = []string{
	0:    "\uFFFD",
	'"':  "&#34;",
	'&':  "&amp;",
	'\'': "&#39;",
	'+':  "&#43;",
	'<':  "&lt;",
	'>':  "&gt;",
}

func htmlReplacer(s string, replacementTable []string) string {
	buf := bufpool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufpool.Put(buf)
	}()
	r, w, written := rune(0), 0, 0
	for i := 0; i < len(s); i += w {
		// Cannot use 'for range s' because we need to preserve the width
		// of the runes in the input. If we see a decoding error, the input
		// width will not be utf8.Runelen(r) and we will overrun the buffer.
		r, w = utf8.DecodeRuneInString(s[i:])
		if int(r) < len(replacementTable) {
			if repl := replacementTable[r]; len(repl) != 0 {
				if written == 0 {
					buf.Grow(len(s))
				}
				buf.WriteString(s[written:i])
				buf.WriteString(repl)
				written = i + w
			}
		}
	}
	if written == 0 {
		return s
	}
	buf.WriteString(s[written:])
	return buf.String()
}

var cssReplacementTable = []string{
	0:    `\0`,
	'\t': `\9`,
	'\n': `\a`,
	'\f': `\c`,
	'\r': `\d`,
	// Encode HTML specials as hex so the output can be embedded
	// in HTML attributes without further encoding.
	'"':  `\22`,
	'&':  `\26`,
	'\'': `\27`,
	'(':  `\28`,
	')':  `\29`,
	'+':  `\2b`,
	'/':  `\2f`,
	':':  `\3a`,
	';':  `\3b`,
	'<':  `\3c`,
	'>':  `\3e`,
	'\\': `\\`,
	'{':  `\7b`,
	'}':  `\7d`,
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func isCSSSpace(b byte) bool {
	switch b {
	case '\t', '\n', '\f', '\r', ' ':
		return true
	}
	return false
}

func cssEscaper(s string) string {
	buf := bufpool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufpool.Put(buf)
	}()
	r, w, written := rune(0), 0, 0
	for i := 0; i < len(s); i += w {
		// See comment in htmlEscaper.
		r, w = utf8.DecodeRuneInString(s[i:])
		var repl string
		switch {
		case int(r) < len(cssReplacementTable) && cssReplacementTable[r] != "":
			repl = cssReplacementTable[r]
		default:
			continue
		}
		if written == 0 {
			buf.Grow(len(s))
		}
		buf.WriteString(s[written:i])
		buf.WriteString(repl)
		written = i + w
		if repl != `\\` && (written == len(s) || isHex(s[written]) || isCSSSpace(s[written])) {
			buf.WriteByte(' ')
		}
	}
	if written == 0 {
		return s
	}
	buf.WriteString(s[written:])
	return buf.String()
}

func decodeCSS(s []byte) []byte {
	i := bytes.IndexByte(s, '\\')
	if i == -1 {
		return s
	}
	// The UTF-8 sequence for a codepoint is never longer than 1 + the
	// number hex digits need to represent that codepoint, so len(s) is an
	// upper bound on the output length.
	b := make([]byte, 0, len(s))
	for len(s) != 0 {
		i := bytes.IndexByte(s, '\\')
		if i == -1 {
			i = len(s)
		}
		b, s = append(b, s[:i]...), s[i:]
		if len(s) < 2 {
			break
		}
		// https://www.w3.org/TR/css3-syntax/#SUBTOK-escape
		// escape ::= unicode | '\' [#x20-#x7E#x80-#xD7FF#xE000-#xFFFD#x10000-#x10FFFF]
		if isHex(s[1]) {
			// https://www.w3.org/TR/css3-syntax/#SUBTOK-unicode
			//   unicode ::= '\' [0-9a-fA-F]{1,6} wc?
			j := 2
			for j < len(s) && j < 7 && isHex(s[j]) {
				j++
			}
			r := hexDecode(s[1:j])
			if r > unicode.MaxRune {
				r, j = r/16, j-1
			}
			n := utf8.EncodeRune(b[len(b):cap(b)], r)
			// The optional space at the end allows a hex
			// sequence to be followed by a literal hex.
			// string(decodeCSS([]byte(`\A B`))) == "\nB"
			b, s = b[:len(b)+n], skipCSSSpace(s[j:])
		} else {
			// `\\` decodes to `\` and `\"` to `"`.
			_, n := utf8.DecodeRune(s[1:])
			b, s = append(b, s[1:1+n]...), s[1+n:]
		}
	}
	return b
}

func hexDecode(s []byte) rune {
	n := '\x00'
	for _, c := range s {
		n <<= 4
		switch {
		case '0' <= c && c <= '9':
			n |= rune(c - '0')
		case 'a' <= c && c <= 'f':
			n |= rune(c-'a') + 10
		case 'A' <= c && c <= 'F':
			n |= rune(c-'A') + 10
		default:
			panic(fmt.Sprintf("Bad hex digit in %q", s))
		}
	}
	return n
}

func skipCSSSpace(c []byte) []byte {
	if len(c) == 0 {
		return c
	}
	// wc ::= #x9 | #xA | #xC | #xD | #x20
	switch c[0] {
	case '\t', '\n', '\f', ' ':
		return c[1:]
	case '\r':
		// This differs from CSS3's wc production because it contains a
		// probable spec error whereby wc contains all the single byte
		// sequences in nl (newline) but not CRLF.
		if len(c) >= 2 && c[1] == '\n' {
			return c[2:]
		}
		return c[1:]
	}
	return c
}

func isCSSNmchar(r rune) bool {
	// Based on the CSS3 nmchar production but ignores multi-rune escape
	// sequences.
	// https://www.w3.org/TR/css3-syntax/#SUBTOK-nmchar
	return 'a' <= r && r <= 'z' ||
		'A' <= r && r <= 'Z' ||
		'0' <= r && r <= '9' ||
		r == '-' ||
		r == '_' ||
		// Non-ASCII cases below.
		0x80 <= r && r <= 0xd7ff ||
		0xe000 <= r && r <= 0xfffd ||
		0x10000 <= r && r <= 0x10ffff
}

var expressionBytes = []byte("expression")
var mozBindingBytes = []byte("mozbinding")

func cssValueFilter(s string) string {
	b, id := decodeCSS([]byte(s)), make([]byte, 0, 64)

	// CSS3 error handling is specified as honoring string boundaries per
	// https://www.w3.org/TR/css3-syntax/#error-handling :
	//     Malformed declarations. User agents must handle unexpected
	//     tokens encountered while parsing a declaration by reading until
	//     the end of the declaration, while observing the rules for
	//     matching pairs of (), [], {}, "", and '', and correctly handling
	//     escapes. For example, a malformed declaration may be missing a
	//     property, colon (:) or value.
	// So we need to make sure that values do not have mismatched bracket
	// or quote characters to prevent the browser from restarting parsing
	// inside a string that might embed JavaScript source.
	for i, c := range b {
		switch c {
		case 0, '"', '\'', '(', ')', '/', ';', '@', '[', '\\', ']', '`', '{', '}':
			return filterFailsafe
		case '-':
			// Disallow <!-- or -->.
			// -- should not appear in valid identifiers.
			if i != 0 && b[i-1] == '-' {
				return filterFailsafe
			}
		default:
			if c < utf8.RuneSelf && isCSSNmchar(rune(c)) {
				id = append(id, c)
			}
		}
	}
	id = bytes.ToLower(id)
	if bytes.Contains(id, expressionBytes) || bytes.Contains(id, mozBindingBytes) {
		return filterFailsafe
	}
	return string(b)
}

type contentType uint8

const (
	contentTypePlain contentType = iota
	contentTypeCSS
	contentTypeHTML
	contentTypeHTMLAttr
	contentTypeJS
	contentTypeJSStr
	contentTypeURL
	contentTypeSrcset
	// contentTypeUnsafe is used in attr.go for values that affect how
	// embedded content and network messages are formed, vetted,
	// or interpreted; or which credentials network messages carry.
	contentTypeUnsafe
)

// attrTypeMap[n] describes the value of the given attribute.
// If an attribute affects (or can mask) the encoding or interpretation of
// other content, or affects the contents, idempotency, or credentials of a
// network message, then the value in this map is contentTypeUnsafe.
// This map is derived from HTML5, specifically
// https://www.w3.org/TR/html5/Overview.html#attributes-1
// as well as "%URI"-typed attributes from
// https://www.w3.org/TR/html4/index/attributes.html
var attrTypeMap = map[string]contentType{
	"accept":          contentTypePlain,
	"accept-charset":  contentTypeUnsafe,
	"action":          contentTypeURL,
	"alt":             contentTypePlain,
	"archive":         contentTypeURL,
	"async":           contentTypeUnsafe,
	"autocomplete":    contentTypePlain,
	"autofocus":       contentTypePlain,
	"autoplay":        contentTypePlain,
	"background":      contentTypeURL,
	"border":          contentTypePlain,
	"checked":         contentTypePlain,
	"cite":            contentTypeURL,
	"challenge":       contentTypeUnsafe,
	"charset":         contentTypeUnsafe,
	"class":           contentTypePlain,
	"classid":         contentTypeURL,
	"codebase":        contentTypeURL,
	"cols":            contentTypePlain,
	"colspan":         contentTypePlain,
	"content":         contentTypeUnsafe,
	"contenteditable": contentTypePlain,
	"contextmenu":     contentTypePlain,
	"controls":        contentTypePlain,
	"coords":          contentTypePlain,
	"crossorigin":     contentTypeUnsafe,
	"data":            contentTypeURL,
	"datetime":        contentTypePlain,
	"default":         contentTypePlain,
	"defer":           contentTypeUnsafe,
	"dir":             contentTypePlain,
	"dirname":         contentTypePlain,
	"disabled":        contentTypePlain,
	"draggable":       contentTypePlain,
	"dropzone":        contentTypePlain,
	"enctype":         contentTypeUnsafe,
	"for":             contentTypePlain,
	"form":            contentTypeUnsafe,
	"formaction":      contentTypeURL,
	"formenctype":     contentTypeUnsafe,
	"formmethod":      contentTypeUnsafe,
	"formnovalidate":  contentTypeUnsafe,
	"formtarget":      contentTypePlain,
	"headers":         contentTypePlain,
	"height":          contentTypePlain,
	"hidden":          contentTypePlain,
	"high":            contentTypePlain,
	"href":            contentTypeURL,
	"hreflang":        contentTypePlain,
	"http-equiv":      contentTypeUnsafe,
	"icon":            contentTypeURL,
	"id":              contentTypePlain,
	"ismap":           contentTypePlain,
	"keytype":         contentTypeUnsafe,
	"kind":            contentTypePlain,
	"label":           contentTypePlain,
	"lang":            contentTypePlain,
	"language":        contentTypeUnsafe,
	"list":            contentTypePlain,
	"longdesc":        contentTypeURL,
	"loop":            contentTypePlain,
	"low":             contentTypePlain,
	"manifest":        contentTypeURL,
	"max":             contentTypePlain,
	"maxlength":       contentTypePlain,
	"media":           contentTypePlain,
	"mediagroup":      contentTypePlain,
	"method":          contentTypeUnsafe,
	"min":             contentTypePlain,
	"multiple":        contentTypePlain,
	"name":            contentTypePlain,
	"novalidate":      contentTypeUnsafe,
	// Skip handler names from
	// https://www.w3.org/TR/html5/webappapis.html#event-handlers-on-elements,-document-objects,-and-window-objects
	// since we have special handling in attrType.
	"open":        contentTypePlain,
	"optimum":     contentTypePlain,
	"pattern":     contentTypeUnsafe,
	"placeholder": contentTypePlain,
	"poster":      contentTypeURL,
	"profile":     contentTypeURL,
	"preload":     contentTypePlain,
	"pubdate":     contentTypePlain,
	"radiogroup":  contentTypePlain,
	"readonly":    contentTypePlain,
	"rel":         contentTypeUnsafe,
	"required":    contentTypePlain,
	"reversed":    contentTypePlain,
	"rows":        contentTypePlain,
	"rowspan":     contentTypePlain,
	"sandbox":     contentTypeUnsafe,
	"spellcheck":  contentTypePlain,
	"scope":       contentTypePlain,
	"scoped":      contentTypePlain,
	"seamless":    contentTypePlain,
	"selected":    contentTypePlain,
	"shape":       contentTypePlain,
	"size":        contentTypePlain,
	"sizes":       contentTypePlain,
	"span":        contentTypePlain,
	"src":         contentTypeURL,
	"srcdoc":      contentTypeHTML,
	"srclang":     contentTypePlain,
	"srcset":      contentTypeSrcset,
	"start":       contentTypePlain,
	"step":        contentTypePlain,
	"style":       contentTypeCSS,
	"tabindex":    contentTypePlain,
	"target":      contentTypePlain,
	"title":       contentTypePlain,
	"type":        contentTypeUnsafe,
	"usemap":      contentTypeURL,
	"value":       contentTypeUnsafe,
	"width":       contentTypePlain,
	"wrap":        contentTypePlain,
	"xmlns":       contentTypeURL,
}

// attrType returns a conservative (upper-bound on authority) guess at the
// type of the lowercase named attribute.
func attrType(name string) contentType {
	if strings.HasPrefix(name, "data-") {
		// Strip data- so that custom attribute heuristics below are
		// widely applied.
		// Treat data-action as URL below.
		name = name[5:]
	} else if colon := strings.IndexRune(name, ':'); colon != -1 {
		if name[:colon] == "xmlns" {
			return contentTypeURL
		}
		// Treat svg:href and xlink:href as href below.
		name = name[colon+1:]
	}
	if t, ok := attrTypeMap[name]; ok {
		return t
	}
	// Treat partial event handler names as script.
	if strings.HasPrefix(name, "on") {
		return contentTypeJS
	}

	// Heuristics to prevent "javascript:..." injection in custom
	// data attributes and custom attributes like g:tweetUrl.
	// https://www.w3.org/TR/html5/dom.html#embedding-custom-non-visible-data-with-the-data-*-attributes
	// "Custom data attributes are intended to store custom data
	//  private to the page or application, for which there are no
	//  more appropriate attributes or elements."
	// Developers seem to store URL content in data URLs that start
	// or end with "URI" or "URL".
	if strings.Contains(name, "src") ||
		strings.Contains(name, "uri") ||
		strings.Contains(name, "url") {
		return contentTypeURL
	}
	return contentTypePlain
}

func htmlNameFilter(s string) string {
	if len(s) == 0 {
		// Avoid violation of structure preservation.
		// <input checked {{.K}}={{.V}}>.
		// Without this, if .K is empty then .V is the value of
		// checked, but otherwise .V is the value of the attribute
		// named .K.
		return filterFailsafe
	}
	s = strings.ToLower(s)
	if t := attrType(s); t != contentTypePlain {
		// TODO: Split attr and element name part filters so we can recognize known attributes.
		return filterFailsafe
	}
	for _, r := range s {
		switch {
		case '0' <= r && r <= '9':
		case 'a' <= r && r <= 'z':
		default:
			return filterFailsafe
		}
	}
	return s
}

func htmlEscaper(s string) string {
	return htmlReplacer(s, htmlReplacementTable)
}

var jsRegexpReplacementTable = []string{
	0:    `\u0000`,
	'\t': `\t`,
	'\n': `\n`,
	'\v': `\u000b`, // "\v" == "v" on IE 6.
	'\f': `\f`,
	'\r': `\r`,
	// Encode HTML specials as hex so the output can be embedded
	// in HTML attributes without further encoding.
	'"':  `\u0022`,
	'$':  `\$`,
	'&':  `\u0026`,
	'\'': `\u0027`,
	'(':  `\(`,
	')':  `\)`,
	'*':  `\*`,
	'+':  `\u002b`,
	'-':  `\-`,
	'.':  `\.`,
	'/':  `\/`,
	'<':  `\u003c`,
	'>':  `\u003e`,
	'?':  `\?`,
	'[':  `\[`,
	'\\': `\\`,
	']':  `\]`,
	'^':  `\^`,
	'{':  `\{`,
	'|':  `\|`,
	'}':  `\}`,
}

var lowUnicodeReplacementTable = []string{
	0: `\u0000`, 1: `\u0001`, 2: `\u0002`, 3: `\u0003`, 4: `\u0004`, 5: `\u0005`, 6: `\u0006`,
	'\a': `\u0007`,
	'\b': `\u0008`,
	'\t': `\t`,
	'\n': `\n`,
	'\v': `\u000b`, // "\v" == "v" on IE 6.
	'\f': `\f`,
	'\r': `\r`,
	0xe:  `\u000e`, 0xf: `\u000f`, 0x10: `\u0010`, 0x11: `\u0011`, 0x12: `\u0012`, 0x13: `\u0013`,
	0x14: `\u0014`, 0x15: `\u0015`, 0x16: `\u0016`, 0x17: `\u0017`, 0x18: `\u0018`, 0x19: `\u0019`,
	0x1a: `\u001a`, 0x1b: `\u001b`, 0x1c: `\u001c`, 0x1d: `\u001d`, 0x1e: `\u001e`, 0x1f: `\u001f`,
}

func replace(s string, replacementTable []string) string {
	var b strings.Builder
	r, w, written := rune(0), 0, 0
	for i := 0; i < len(s); i += w {
		// See comment in htmlEscaper.
		r, w = utf8.DecodeRuneInString(s[i:])
		var repl string
		switch {
		case int(r) < len(lowUnicodeReplacementTable):
			repl = lowUnicodeReplacementTable[r]
		case int(r) < len(replacementTable) && replacementTable[r] != "":
			repl = replacementTable[r]
		case r == '\u2028':
			repl = `\u2028`
		case r == '\u2029':
			repl = `\u2029`
		default:
			continue
		}
		if written == 0 {
			b.Grow(len(s))
		}
		b.WriteString(s[written:i])
		b.WriteString(repl)
		written = i + w
	}
	if written == 0 {
		return s
	}
	b.WriteString(s[written:])
	return b.String()
}

func jsRegexpEscaper(s string) string {
	s = replace(s, jsRegexpReplacementTable)
	if s == "" {
		// /{{.X}}/ should not produce a line comment when .X == "".
		return "(?:)"
	}
	return s
}

var jsStrReplacementTable = []string{
	0:    `\u0000`,
	'\t': `\t`,
	'\n': `\n`,
	'\v': `\u000b`, // "\v" == "v" on IE 6.
	'\f': `\f`,
	'\r': `\r`,
	// Encode HTML specials as hex so the output can be embedded
	// in HTML attributes without further encoding.
	'"':  `\u0022`,
	'&':  `\u0026`,
	'\'': `\u0027`,
	'+':  `\u002b`,
	'/':  `\/`,
	'<':  `\u003c`,
	'>':  `\u003e`,
	'\\': `\\`,
}

func jsStrEscaper(s string) string {
	return replace(s, jsStrReplacementTable)
}

var htmlNospaceReplacementTable = []string{
	0:    "&#xfffd;",
	'\t': "&#9;",
	'\n': "&#10;",
	'\v': "&#11;",
	'\f': "&#12;",
	'\r': "&#13;",
	' ':  "&#32;",
	'"':  "&#34;",
	'&':  "&amp;",
	'\'': "&#39;",
	'+':  "&#43;",
	'<':  "&lt;",
	'=':  "&#61;",
	'>':  "&gt;",
	// A parse error in the attribute value (unquoted) and
	// before attribute value states.
	// Treated as a quoting character by IE.
	'`': "&#96;",
}

func htmlNospaceEscaper(s string) string {
	return htmlReplacer(s, htmlNospaceReplacementTable)
}

func filterSrcsetElement(s string, left int, right int, b *bytes.Buffer) {
	start := left
	for start < right && isHTMLSpace(s[start]) {
		start++
	}
	end := right
	for i := start; i < right; i++ {
		if isHTMLSpace(s[i]) {
			end = i
			break
		}
	}
	if url := s[start:end]; isSafeURL(url) {
		// If image metadata is only spaces or alnums then
		// we don't need to URL normalize it.
		metadataOk := true
		for i := end; i < right; i++ {
			if !isHTMLSpaceOrASCIIAlnum(s[i]) {
				metadataOk = false
				break
			}
		}
		if metadataOk {
			b.WriteString(s[left:start])
			processURLOnto(url, true, b)
			b.WriteString(s[end:right])
			return
		}
	}
	b.WriteString("#")
	b.WriteString(filterFailsafe)
}

const htmlSpaceAndASCIIAlnumBytes = "\x00\x36\x00\x00\x01\x00\xff\x03\xfe\xff\xff\x07\xfe\xff\xff\x07"

func isHTMLSpace(c byte) bool {
	return (c <= 0x20) && 0 != (htmlSpaceAndASCIIAlnumBytes[c>>3]&(1<<uint(c&0x7)))
}

func isHTMLSpaceOrASCIIAlnum(c byte) bool {
	return (c < 0x80) && 0 != (htmlSpaceAndASCIIAlnumBytes[c>>3]&(1<<uint(c&0x7)))
}

func isSafeURL(s string) bool {
	if i := strings.IndexRune(s, ':'); i >= 0 && !strings.ContainsRune(s[:i], '/') {

		protocol := s[:i]
		if !strings.EqualFold(protocol, "http") && !strings.EqualFold(protocol, "https") && !strings.EqualFold(protocol, "mailto") {
			return false
		}
	}
	return true
}

func processURLOnto(s string, norm bool, b *bytes.Buffer) bool {
	b.Grow(len(s) + 16)
	written := 0
	// The byte loop below assumes that all URLs use UTF-8 as the
	// content-encoding. This is similar to the URI to IRI encoding scheme
	// defined in section 3.1 of  RFC 3987, and behaves the same as the
	// EcmaScript builtin encodeURIComponent.
	// It should not cause any misencoding of URLs in pages with
	// Content-type: text/html;charset=UTF-8.
	for i, n := 0, len(s); i < n; i++ {
		c := s[i]
		switch c {
		// Single quote and parens are sub-delims in RFC 3986, but we
		// escape them so the output can be embedded in single
		// quoted attributes and unquoted CSS url(...) constructs.
		// Single quotes are reserved in URLs, but are only used in
		// the obsolete "mark" rule in an appendix in RFC 3986
		// so can be safely encoded.
		case '!', '#', '$', '&', '*', '+', ',', '/', ':', ';', '=', '?', '@', '[', ']':
			if norm {
				continue
			}
		// Unreserved according to RFC 3986 sec 2.3
		// "For consistency, percent-encoded octets in the ranges of
		// ALPHA (%41-%5A and %61-%7A), DIGIT (%30-%39), hyphen (%2D),
		// period (%2E), underscore (%5F), or tilde (%7E) should not be
		// created by URI producers
		case '-', '.', '_', '~':
			continue
		case '%':
			// When normalizing do not re-encode valid escapes.
			if norm && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
				continue
			}
		default:
			// Unreserved according to RFC 3986 sec 2.3
			if 'a' <= c && c <= 'z' {
				continue
			}
			if 'A' <= c && c <= 'Z' {
				continue
			}
			if '0' <= c && c <= '9' {
				continue
			}
		}
		b.WriteString(s[written:i])
		fmt.Fprintf(b, "%%%02x", c)
		written = i + 1
	}
	b.WriteString(s[written:])
	return written != 0
}

func srcsetFilterAndEscaper(s string) string {
	buf := bufpool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufpool.Put(buf)
	}()
	written := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			filterSrcsetElement(s, written, i, buf)
			buf.WriteString(",")
			written = i + 1
		}
	}
	filterSrcsetElement(s, written, len(s), buf)
	return buf.String()
}

func urlProcessor(norm bool, s string) string {
	buf := bufpool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufpool.Put(buf)
	}()
	if processURLOnto(s, norm, buf) {
		return buf.String()
	}
	return s
}

func urlEscaper(s string) string {
	return urlProcessor(false, s)
}

func urlFilter(s string) string {
	if !isSafeURL(s) {
		return "#" + filterFailsafe
	}
	return s
}
