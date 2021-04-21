package hy

import "github.com/microcosm-cc/bluemonday"

type Sanitizer interface {
	Sanitize(string) string
}

var defaultSanitizer = configDefaultSanitizer(bluemonday.UGCPolicy())

var attributesMap = map[string][]string{
	"form": {"accept-charset", "autocomplete", "name", "rel", "action", "enctype", "method", "novalidate", "target"},
	"input": {
		"accept", "alt", "autocomplete", "autofocus", "capture", "checked", "dirname", "disabled", "form",
		"formaction", "formenctype", "formmethod", "formnovalidate", "formtarget", "height", "list", "max",
		"maxlength", "min", "minlength", "multiple", "name", "pattern", "placeholder", "readonly", "required",
		"size", "src", "step", "type", "value", "width",
	},
	"button": {
		"autofocus", "disabled", "form", "formaction", "formenctype",
		"formmethod", "formnovalidate", "formtarget", "name", "type", "value",
	},
	"label":    {"for"},
	"select":   {"autocomplete", "autofocus", "disabled", "form", "multiple", "name", "required", "size"},
	"option":   {"disabled", "label", "selected", "value"},
	"optgroup": {"label", "disabled"},
	"link": {
		"as", "crossorigin", "disabled", "href", "hreflang", "imagesizes", "imagesrcset", "media", "rel",
		"sizes", "title", "type",
	},
	"script":   {"async", "crossorigin", "defer", "integrity", "nomodule", "nonce", "referrerpolicy", "src", "type"},
	"pre":      {},
	"a":        {"href", "hreflang", "ping", "rel", "target", "type"},
	"fieldset": {"disabled", "form", "name"},
	"legend":   {},
	"textarea": {
		"autocomplete", "autofocus", "cols", "disabled", "form", "maxlength", "name",
		"placeholder", "readonly", "required", "rows", "spellcheck", "wrap",
	},
}

func AttributesMap() map[string][]string {
	m := make(map[string][]string)
	for k, v := range attributesMap {
		m[k] = v
	}
	return m
}

func configDefaultSanitizer(p *bluemonday.Policy) *bluemonday.Policy {
	defaultSanitizerTags := []string{
		"form", "input", "button", "label", "select", "option", "optgroup", "pre", "a", "fieldset", "legend", "textarea",
	}
	p.AllowStyling()
	p.AllowImages()
	p.AllowLists()
	p.AllowTables()
	defer p.RequireNoFollowOnLinks(false) // deferred til the last because bluemonday looooves to turn it back on
	for _, tag := range defaultSanitizerTags {
		p.AllowAttrs(attributesMap[tag]...).OnElements(tag)
	}
	return p
}

func Allow(tags ...string) Sanitizer {
	p := bluemonday.UGCPolicy()
	configDefaultSanitizer(p)
	for _, tag := range tags {
		p.AllowAttrs(attributesMap[tag]...).OnElements(tag)
	}
	return p
}
