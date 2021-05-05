package hy

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

var globalAttributes = map[string]struct{}{
	"accesskey": {}, "autocapitalize": {}, "class": {}, "contenteditable": {}, "dir": {}, "draggable": {},
	"enterkeyhint": {}, "hidden": {}, "id": {}, "inputmode": {}, "is": {}, "itemid": {}, "itemprop": {},
	"itemref": {}, "itemscope": {}, "itemtype": {}, "lang": {}, "nonce": {}, "part": {}, "slot": {},
	"spellcheck": {} /*, "style": {}*/, "tabindex": {}, "title": {}, "translate": {},
}

var tagAttributes = map[string]map[string]struct{}{
	"html": {"xmlns": {}},
	"base": {"href": {}, "target": {}},
	"link": {
		"as": {}, "crossorigin": {}, "disabled": {}, "href": {}, "hreflang": {}, "imagesizes": {}, "imagesrcset": {},
		"integrity": {}, "media": {}, "prefetch": {}, "referrerpolicy": {}, "rel": {}, "sizes": {}, "title": {}, "type": {},
	},
	"meta":       {"charset": {}, "content": {}, "http-equiv": {}, "name": {}},
	"style":      {"type": {}, "media": {}, "nonce": {}, "title": {}},
	"blockquote": {"cite": {}},
	"hr":         {"color": {}},
	"li":         {"value": {}},
	"ol":         {"reversed": {}, "start": {}, "type": {}},
	"a":          {"download": {}, "href": {}, "hreflang": {}, "ping": {}, "referrerpolicy": {}, "rel": {}, "target": {}, "type": {}},
	"data":       {"value": {}},
	"q":          {"cite": {}},
	"time":       {"datetime": {}},
	"area": {
		"alt": {}, "coords": {}, "download": {}, "href": {}, "hreflang": {}, "ping": {}, "referrerpolicy": {},
		"rel": {}, "shape": {}, "target": {},
	},
	"audio": {
		"autoplay": {}, "controls": {}, "crossorigin": {}, "currentTime": {}, "disableRemotePlayback": {},
		"loop": {}, "muted": {}, "preload": {}, "src": {},
	},
	"img": {
		"alt": {}, "crossorigin": {}, "decoding": {}, "height": {}, "ismap": {}, "loading": {}, "referrerpolicy": {},
		"sizes": {}, "src": {}, "srcset": {}, "width": {}, "usemap": {},
	},
	"map":   {"name": {}},
	"track": {"default": {}, "kind": {}, "label": {}, "src": {}, "srclang": {}},
	"video": {
		"autoplay": {}, "autoPictureInPicture": {}, "buffered": {}, "controls": {}, "controlslist": {},
		"crossorigin": {}, "currentTime": {}, "disabledPictureInPicture": {}, "disableRemotePlayback": {}, "height": {},
		"loop": {}, "muted": {}, "playsinline": {}, "poster": {}, "preload": {}, "src": {}, "width": {},
	},
	"embed": {"height": {}, "src": {}, "type": {}, "width": {}},
	"iframe": {
		"allow": {}, "allowfullscreen": {}, "allowpaymentrequest": {}, "csp": {}, "height": {}, "loading": {},
		"name": {}, "referrerpolicy": {}, "sandbox": {}, "src": {}, "srcdoc": {}, "width": {},
	},
	"object": {"data": {}, "form": {}, "height": {}, "name": {}, "type": {}, "usemap": {}, "width": {}},
	"param":  {"name": {}, "value": {}},
	"portal": {"referrerpolicy": {}, "src": {}},
	"source": {"media": {}, "sizes": {}, "src": {}, "srcset": {}, "type": {}},
	"canvas": {"height": {}, "width": {}},
	"script": {
		"async": {}, "crossorigin": {}, "defer": {}, "integrity": {}, "nomodule": {}, "nonce": {}, "referrerpolicy": {},
		"src": {}, "type": {},
	},
	"del":      {"cite": {}, "datetime": {}},
	"ins":      {"cite": {}, "datetime": {}},
	"col":      {"span": {}},
	"colgroup": {"span": {}},
	"td":       {"colspan": {}, "headers": {}, "rowspan": {}},
	"th":       {"abbr": {}, "colspan": {}, "headers": {}, "rowspan": {}, "scope": {}},
	"button": {
		"autofocus": {}, "disabled": {}, "form": {}, "formaction": {}, "formenctype": {}, "formmethod": {},
		"formnovalidate": {}, "formtarget": {}, "name": {}, "type": {}, "value": {},
	},
	"fieldset": {"disabled": {}, "form": {}, "name": {}},
	"form": {
		"accept-charset": {}, "autocomplete": {}, "name": {}, "rel": {}, "action": {}, "enctype": {}, "method": {},
		"novalidate": {}, "target": {},
	},
	"input": {
		"accept": {}, "alt": {}, "autocomplete": {}, "autofocus": {}, "capture": {}, "checked": {}, "dirname": {},
		"disabled": {}, "form": {}, "formaction": {}, "formenctype": {}, "formmethod": {}, "formnovalidate": {},
		"formtarget": {}, "height": {}, "list": {}, "max": {}, "maxlength": {}, "min": {}, "minlength": {},
		"multiple": {}, "name": {}, "pattern": {}, "placeholder": {}, "readonly": {}, "required": {}, "size": {},
		"src": {}, "step": {}, "type": {}, "value": {}, "width": {},
	},
	"label":    {"for": {}},
	"meter":    {"value": {}, "min": {}, "max": {}, "low": {}, "high": {}, "optimum": {}, "form": {}},
	"optgroup": {"disabled": {}, "label": {}},
	"option":   {"disabled": {}, "label": {}, "selected": {}, "value": {}},
	"output":   {"for": {}, "form": {}, "name": {}},
	"progress": {"max": {}, "value": {}},
	"select": {
		"autocomplete": {}, "autofocus": {}, "disabled": {}, "form": {}, "multiple": {}, "name": {},
		"required": {}, "size": {},
	},
	"textarea": {
		"autocomplete": {}, "autofocus": {}, "cols": {}, "disabled": {}, "form": {}, "maxlength": {}, "name": {},
		"placeholder": {}, "readonly": {}, "required": {}, "rows": {}, "spellcheck": {}, "wrap": {},
	},
	"details": {"open": {}},
	"dialog":  {"open": {}},
	"menu":    {"type": {}},
	"slot":    {"name": {}},
}

var allowedTags = map[string]struct{}{
	"html": {}, "base": {}, "link": {}, "meta": {} /*, "style": {}*/, "title": {}, "body": {}, "address": {},
	"article": {}, "aside": {}, "footer": {}, "header": {}, "h1": {}, "h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {},
	"main": {}, "nav": {}, "section": {}, "blockquote": {}, "dd": {}, "div": {}, "dl": {}, "dt": {}, "figcaption": {},
	"figure": {}, "hr": {}, "li": {}, "ol": {}, "p": {}, "pre": {}, "ul": {}, "a": {}, "abbr": {}, "b": {}, "bdi": {},
	"bdo": {}, "br": {}, "cite": {}, "code": {}, "data": {}, "dfn": {}, "em": {}, "i": {}, "kbd": {}, "mark": {},
	"q": {}, "rp": {}, "rt": {}, "ruby": {}, "s": {}, "samp": {}, "small": {}, "span": {}, "strong": {}, "sub": {},
	"sup": {}, "time": {}, "u": {}, "var": {}, "wbr": {}, "area": {}, "audio": {}, "img": {}, "map": {}, "track": {},
	"video": {}, "embed": {} /*, "iframe": {}*/, "object": {}, "param": {}, "picture": {}, "portal": {}, "source": {},
	"svg": {}, "math": {}, "canvas": {}, "noscript": {} /*, "script": {}*/, "del": {}, "ins": {}, "caption": {},
	"col": {}, "colgroup": {}, "table": {}, "tbody": {}, "td": {}, "tfoot": {}, "th": {}, "thead": {}, "tr": {},
	"button": {}, "datalist": {}, "fieldset": {}, "form": {}, "input": {}, "label": {}, "legend": {}, "meter": {},
	"optgroup": {}, "option": {}, "output": {}, "progress": {}, "select": {}, "textarea": {}, "details": {},
	"dialog": {}, "menu": {}, "summary": {}, "slot": {}, "template": {},
}

func Allow(tag string, attributeName string, attributeValue string) bool {
	tag = strings.ToLower(tag)
	_, ok := allowedTags[tag]
	if !ok {
		return false
	}
	if attributeName == "" {
		return ok
	}
	attributeName = strings.ToLower(attributeName)
	// TODO: need to handle URL/srcset attributes here. Filter out data:* and javascript:* values.
	if _, ok = globalAttributes[attributeName]; ok {
		return true
	}
	if strings.HasPrefix(attributeName, "data-") || tag == "svg" || tag == "math" {
		return true
	}
	if _, ok = tagAttributes[tag][attributeName]; ok {
		return true
	}
	return false
}

var htmlReplacementTableV2 = []string{
	0:    "\uFFFD",
	'"':  "&#34;",
	'&':  "&amp;",
	'\'': "&#39;",
	'+':  "&#43;",
	'<':  "&lt;",
	'>':  "&gt;",
}

var htmlAttrReplacementTable = []string{
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

func escapeRunes(buf *bytes.Buffer, replacementTable []string, s string) {
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
		buf.WriteString(s)
		return
	}
	buf.WriteString(s[written:])
}

var urlAttrNames = map[string]struct{}{
	"action": {}, "archive": {}, "background": {}, "cite": {}, "classid": {}, "codebase": {}, "data": {},
	"formaction": {}, "href": {}, "icon": {}, "longdesc": {}, "manifest": {}, "poster": {}, "profile": {}, "src": {},
	"usemap": {}, "xmlns": {},
}

func isURLAttr(name string) bool {
	return false
}

// processURLOnto
func escapeURL(buf *bytes.Buffer, s string) {
	buf.Grow(len(s) + 16)
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
			break
		// Unreserved according to RFC 3986 sec 2.3
		// "For consistency, percent-encoded octets in the ranges of
		// ALPHA (%41-%5A and %61-%7A), DIGIT (%30-%39), hyphen (%2D),
		// period (%2E), underscore (%5F), or tilde (%7E) should not be
		// created by URI producers
		case '-', '.', '_', '~':
			continue
		case '%':
			break
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
		buf.WriteString(s[written:i])
		fmt.Fprintf(buf, "%%%02x", c)
		written = i + 1
	}
	buf.WriteString(s[written:])
}

func escapeSrcset(buf *bytes.Buffer, s string) {
}
