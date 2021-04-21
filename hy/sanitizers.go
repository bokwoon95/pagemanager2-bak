package hy

import "github.com/microcosm-cc/bluemonday"

type Sanitizer interface {
	Sanitize(string) string
}

var defaultSanitizer = func() Sanitizer {
	p := bluemonday.UGCPolicy()
	p.AllowStyling()
	p.AllowDataAttributes()
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/form#attributes
	p.AllowElements("form")
	p.AllowAttrs("accept-charset", "autocomplete", "name", "rel", "action", "enctype", "method", "novalidate", "target").OnElements("form")
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input#attributes
	p.AllowElements("input")
	p.AllowAttrs(
		"accept", "alt", "autocomplete", "autofocus", "capture", "checked",
		"dirname", "disabled", "form", "formaction", "formenctype", "formmethod",
		"formnovalidate", "formtarget", "height", "list", "max", "maxlength", "min",
		"minlength", "multiple", "name", "pattern", "placeholder", "readonly",
		"required", "size", "src", "step", "type", "value", "width",
	).OnElements("input")
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/button#attributes
	p.AllowElements("button")
	p.AllowAttrs(
		"autofocus", "disabled", "form", "formaction", "formenctype",
		"formmethod", "formnovalidate", "formtarget", "name", "type", "value",
	).OnElements("button")
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/label#attributes
	p.AllowElements("label")
	p.AllowAttrs("for").OnElements("label")
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/select#attributes
	p.AllowElements("select")
	p.AllowAttrs("autocomplete", "autofocus", "disabled", "form", "multiple", "name", "required", "size").OnElements("select")
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/option#attributes
	p.AllowElements("option")
	p.AllowAttrs("disabled", "label", "selected", "value").OnElements("option")
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/optgroup#attributes
	p.AllowElements("optgroup")
	p.AllowAttrs("label", "disabled").OnElements("optgroup")
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Global_attributes/inputmode
	p.AllowAttrs("inputmode").Globally()
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/link#attributes
	p.AllowElements("link")
	p.AllowAttrs(
		"as", "crossorigin", "disabled", "href", "hreflang", "imagesizes",
		"imagesrcset", "media", "rel", "sizes", "title", "type",
	).OnElements("link")
	p.AllowStandardURLs()
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/script#attributes
	p.AllowElements("script")
	p.AllowAttrs("async", "crossorigin", "defer", "integrity", "nomodule", "nonce", "referrerpolicy", "src", "type").OnElements("script")
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/pre#attributes
	p.AllowElements("pre")
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/a#attributes
	p.AllowElements("a")
	p.AllowAttrs("href", "hreflang", "ping", "rel", "target", "type").OnElements("a")
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/fieldset#attributes
	p.AllowElements("fieldset", "legend")
	p.AllowAttrs("disabled", "form", "name").OnElements("fieldset")
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/textarea#attributes
	p.AllowElements("textarea")
	p.AllowAttrs(
		"autocomplete", "autofocus", "cols", "disabled", "form", "maxlength", "name",
		"placeholder", "readonly", "required", "rows", "spellcheck", "wrap",
	).OnElements("textarea")

	p.AllowElements("svg")

	p.AllowImages()
	p.AllowLists()
	p.AllowTables()

	// settings which bluemonday loves to turn back on, leave it for the last
	p.RequireNoFollowOnLinks(false)
	return p
}()
