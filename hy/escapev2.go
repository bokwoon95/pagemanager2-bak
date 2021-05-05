package hy

import (
	"strings"
)

var allowedTags = map[string]struct{}{
	"article": {}, "aside": {}, "details": {}, "figure": {}, "section": {}, "summary": {}, "h1": {}, "h2": {}, "h3": {},
	"h4": {}, "h5": {}, "h6": {}, "hgroup": {}, "blockquote": {}, "br": {}, "div": {}, "hr": {}, "p": {}, "span": {}, "wbr": {},
	"a": {}, "area": {}, "img": {}, "abbr": {}, "acronym": {}, "cite": {}, "code": {}, "dfn": {}, "em": {}, "figcaption": {},
	"mark": {}, "s": {}, "samp": {}, "strong": {}, "sub": {}, "sup": {}, "var": {}, "q": {}, "time": {}, "b": {}, "i": {},
	"pre": {}, "small": {}, "strike": {}, "tt": {}, "u": {}, "bdi": {}, "bdo": {}, "rp": {}, "rt": {}, "ruby": {}, "del": {},
	"ins": {}, "ol": {}, "ul": {}, "li": {}, "dl": {}, "dt": {}, "dd": {}, "table": {}, "caption": {}, "col": {}, "colgroup": {},
	"thead": {}, "tr": {}, "td": {}, "th": {}, "tbody": {}, "tfoot": {}, "meter": {}, "progress": {},
}

var globalAttributes = map[string]struct{}{
	"accesskey": {}, "autocapitalize": {}, "class": {}, "contenteditable": {}, "dir": {}, "draggable": {}, "enterkeyhint": {},
	"hidden": {}, "id": {}, "inputmode": {}, "is": {}, "itemid": {}, "itemprop": {}, "itemref": {}, "itemscope": {}, "itemtype": {},
	"lang": {}, "nonce": {}, "part": {}, "slot": {}, "spellcheck": {} /*, "style": {}*/, "tabindex": {}, "title": {}, "translate": {},
}

var tagAttributes = map[string]map[string]struct{}{
	"html": {"xmlns": {}},
	"base": {"href": {}, "target": {}},
	"head": {},
	"link": {
		"as": {}, "crossorigin": {}, "disabled": {}, "href": {}, "hreflang": {}, "imagesizes": {}, "imagesrcset": {}, "integrity": {},
		"media": {}, "prefetch": {}, "referrerpolicy": {}, "rel": {}, "sizes": {}, "title": {}, "type": {},
	},
	"meta": {"charset": {}, "content": {}, "http-equiv": {}, "name": {}},
}

func DefaultFilter(tag string, attributeName string, attributeValue string) bool {
	_, ok := allowedTags[strings.ToLower(tag)]
	if !ok {
		return false
	}
	if attributeName == "" {
		return ok
	}
	if _, ok = globalAttributes[attributeName]; ok {
		return true
	}
	if strings.HasPrefix(attributeName, "data-") {
		return true
	}
	if _, ok = tagAttributes[tag][attributeName]; ok {
		return true
	}
	return false
}
