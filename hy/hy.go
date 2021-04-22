// Package hy implements hyperscript in Go.
package hy

import (
	"fmt"
	"html/template"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
)

const (
	Enabled  = "\x00"
	Disabled = "\x01"
)

type Element interface{ AppendHTML(*strings.Builder) error }

type Sanitizer interface{ Sanitize(string) string }

// https://developer.mozilla.org/en-US/docs/Glossary/Empty_element
var singletonElements = map[string]struct{}{
	"AREA": {}, "BASE": {}, "BR": {}, "COL": {}, "EMBED": {}, "HR": {}, "IMG": {}, "INPUT": {},
	"LINK": {}, "META": {}, "PARAM": {}, "SOURCE": {}, "TRACK": {}, "WBR": {},
}

var bufpool = sync.Pool{New: func() interface{} { return &strings.Builder{} }}

var (
	defaultSanitizer = NewSanitizer()
	unsafeSanitizer  = NewSanitizer("script", "link")
)

func DefaultSanitizer() Sanitizer { return defaultSanitizer }
func UnsafeSanitizer() Sanitizer  { return unsafeSanitizer }

// attributesMap is the list of attributes that we allow for each tag.
// Attributes were referenced from MDN docs, so should be comprehensive.
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

type HTMLElement struct {
	attrs    Attributes
	children []Element
}

func H(selector string, attributes map[string]string, children ...Element) HTMLElement {
	return HTMLElement{
		attrs:    ParseAttributes(selector, attributes),
		children: children,
	}
}

func (el *HTMLElement) Set(selector string, attributes map[string]string, children ...Element) {
	el.attrs = ParseAttributes(selector, attributes)
	el.children = children
}

func (el *HTMLElement) Append(selector string, attributes map[string]string, children ...Element) {
	el.children = append(el.children, H(selector, attributes, children...))
}

func (el *HTMLElement) AppendElements(elements ...Element) {
	el.children = append(el.children, elements...)
}

func (el HTMLElement) Tag() string { return el.attrs.Tag }

func (el HTMLElement) ID() string { return el.attrs.ID }

func (el HTMLElement) AppendHTML(buf *strings.Builder) error {
	err := AppendHTML(buf, el.attrs, el.children)
	if err != nil {
		return err
	}
	return nil
}

type textValue struct {
	values []interface{}
	unsafe bool
}

func Txt(a ...interface{}) Element {
	return textValue{values: a}
}

func UnsafeTxt(a ...interface{}) Element {
	return textValue{values: a, unsafe: true}
}

func (txt textValue) AppendHTML(buf *strings.Builder) error {
	for i, value := range txt.values {
		switch value := value.(type) {
		case string:
			if value == "" {
				continue
			}
			if txt.unsafe {
				buf.WriteString(value)
			} else {
				template.HTMLEscape(buf, []byte(value))
			}
			if strings.TrimSpace(value) == "" {
				continue
			}
			r, _ := utf8.DecodeLastRuneInString(value)
			if i != len(txt.values)-1 && !unicode.IsSpace(r) {
				buf.WriteByte(' ')
			}
		default:
			if txt.unsafe {
				buf.WriteString(Stringify(value))
			} else {
				template.HTMLEscape(buf, []byte(Stringify(value)))
			}
		}
	}
	return nil
}

type Elements []Element

func (l *Elements) Append(selector string, attributes map[string]string, children ...Element) {
	*l = append(*l, H(selector, attributes, children...))
}

func (l *Elements) AppendElements(children ...Element) {
	*l = append(*l, children...)
}

func (l Elements) AppendHTML(buf *strings.Builder) error {
	var err error
	for _, el := range l {
		err = el.AppendHTML(buf)
		if err != nil {
			return err
		}
	}
	return nil
}

type Attr map[string]string

type Attributes struct {
	ParseErr error
	Tag      string
	ID       string
	Class    string
	Dict     map[string]string
}

func ParseAttributes(selector string, attributes map[string]string) Attributes {
	type State int
	const (
		StateNone State = iota
		StateTag
		StateID
		StateClass
		StateAttrName
		StateAttrValue
	)
	attrs := Attributes{Dict: make(map[string]string)}
	defer func() {
		if attrs.ParseErr != nil {
			for k, v := range attributes {
				attrs.Dict[k] = v
			}
		}
	}()
	state := StateTag
	var classes []string
	var name []rune
	var value []rune
	for i, c := range selector {
		if c == '#' || c == '.' || c == '[' {
			switch state {
			case StateTag:
				attrs.Tag = string(value)
			case StateID:
				attrs.ID = string(value)
			case StateClass:
				if len(value) > 0 {
					classes = append(classes, string(value))
				}
			case StateAttrName, StateAttrValue:
				attrs.ParseErr = fmt.Errorf("unclosed attribute: position=%d char=%c selector=%s", i, c, selector)
				return attrs
			}
			value = value[:0]
			switch c {
			case '#':
				state = StateID
			case '.':
				state = StateClass
			case '[':
				state = StateAttrName
			}
			continue
		}
		if c == '=' {
			switch state {
			case StateAttrName:
				state = StateAttrValue
			default:
				attrs.ParseErr = fmt.Errorf("unopened attribute: position=%d char=%c selector=%s", i, c, selector)
				return attrs
			}
			continue
		}
		if c == ']' {
			switch state {
			case StateAttrName:
				if _, ok := attrs.Dict[string(name)]; ok {
					break
				}
				attrs.Dict[string(name)] = Enabled
			case StateAttrValue:
				if _, ok := attrs.Dict[string(name)]; ok {
					break
				}
				attrs.Dict[string(name)] = string(value)
			default:
				attrs.ParseErr = fmt.Errorf("unopened attribute: position=%d char=%c selector=%s", i, c, selector)
				return attrs
			}
			name = name[:0]
			value = value[:0]
			state = StateNone
			continue
		}
		switch state {
		case StateTag, StateID, StateClass, StateAttrValue:
			value = append(value, c)
		case StateAttrName:
			name = append(name, c)
		case StateNone:
			attrs.ParseErr = fmt.Errorf("unknown state (please prepend with '#', '.' or '['): position=%d char=%c selector=%s", i, c, selector)
			return attrs
		}
	}
	// flush value
	if len(value) > 0 {
		switch state {
		case StateTag:
			attrs.Tag = string(value)
		case StateID:
			attrs.ID = string(value)
		case StateClass:
			classes = append(classes, string(value))
		case StateNone: // do nothing i.e. drop the value
		case StateAttrName, StateAttrValue:
			attrs.ParseErr = fmt.Errorf("unclosed attribute: selector=%s", selector)
			return attrs
		}
		value = value[:0]
	}
	if len(classes) > 0 {
		attrs.Class = strings.Join(classes, " ")
	}
	for name, value := range attributes {
		switch name {
		case "id":
			attrs.ID = value
		case "class":
			if value != "" {
				if attrs.Class == "" {
					attrs.Class = value
				} else {
					attrs.Class += " " + value
				}
			}
		default:
			attrs.Dict[name] = value
		}
	}
	return attrs
}

func AppendHTML(buf *strings.Builder, attrs Attributes, children []Element) error {
	var err error
	if attrs.ParseErr != nil {
		return attrs.ParseErr
	}
	if attrs.Tag != "" {
		buf.WriteString(`<`)
		template.HTMLEscape(buf, []byte(attrs.Tag))
	} else {
		buf.WriteString(`<div`)
	}
	AppendAttributes(buf, attrs)
	buf.WriteString(`>`)
	if _, ok := singletonElements[strings.ToUpper(attrs.Tag)]; !ok {
		for _, child := range children {
			err = child.AppendHTML(buf)
			if err != nil {
				return err
			}
		}
		buf.WriteString(`</`)
		template.HTMLEscape(buf, []byte(attrs.Tag))
		buf.WriteString(`>`)
	}
	return nil
}

func AppendAttributes(buf *strings.Builder, attrs Attributes) {
	if attrs.ID != "" {
		buf.WriteString(` id="`)
		template.HTMLEscape(buf, []byte(attrs.ID))
		buf.WriteString(`"`)
	}
	if attrs.Class != "" {
		buf.WriteString(` class="`)
		template.HTMLEscape(buf, []byte(attrs.Class))
		buf.WriteString(`"`)
	}
	var names []string
	for name := range attrs.Dict {
		switch name {
		case "id", "class": // skip
		default:
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if name == "" {
			continue
		}
		value := attrs.Dict[name]
		switch value {
		case Enabled:
			buf.WriteString(` `)
			template.HTMLEscape(buf, []byte(name))
		case Disabled:
			continue
		default:
			buf.WriteString(` `)
			template.HTMLEscape(buf, []byte(name))
			buf.WriteString(`="`)
			template.HTMLEscape(buf, []byte(value))
			buf.WriteString(`"`)
		}
	}
}

func Marshal(s Sanitizer, el Element) (template.HTML, error) {
	buf := bufpool.Get().(*strings.Builder)
	defer func() {
		buf.Reset()
		bufpool.Put(buf)
	}()
	err := el.AppendHTML(buf)
	if err != nil {
		return "", err
	}
	if s == nil {
		s = DefaultSanitizer()
	}
	output := s.Sanitize(buf.String())
	return template.HTML(output), nil
}

// adapted from database/sql:asString, text/template:printableValue,printValue
func Stringify(v interface{}) string {
	switch v := v.(type) {
	case fmt.Stringer:
		return v.String()
	case string:
		return v
	case []byte:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 64)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	rv := reflect.ValueOf(v)
	for {
		if rv.Kind() != reflect.Ptr && rv.Kind() != reflect.Interface {
			break
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return "<no value>"
	}
	if rv.Kind() == reflect.Chan {
		return "<channel>"
	}
	if rv.Kind() == reflect.Func {
		return "<function>"
	}
	return fmt.Sprint(v)
}

func NewSanitizer(allowedTags ...string) Sanitizer {
	defaultSanitizerTags := []string{
		"form", "input", "button", "label", "select", "option", "optgroup", "pre", "a", "fieldset", "legend", "textarea",
	}
	p := bluemonday.UGCPolicy()
	p.AllowStyling()
	p.AllowImages()
	p.AllowLists()
	p.AllowTables()
	p.AllowAttrs("inputmode", "hidden").Globally()
	defer p.RequireNoFollowOnLinks(false) // deferred til the last because bluemonday looooves to turn it back on
	for _, tag := range defaultSanitizerTags {
		p.AllowAttrs(attributesMap[tag]...).OnElements(tag)
	}
	for _, tag := range allowedTags {
		p.AllowAttrs(attributesMap[tag]...).OnElements(tag)
	}
	return p
}
