// Package hy implements hyperscript in Go.
package hy

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
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

type Element interface{ WriteHTML(io.Writer) error }

// https://developer.mozilla.org/en-US/docs/Glossary/Empty_element
var singletonElements = map[string]struct{}{
	"area": {}, "base": {}, "br": {}, "col": {}, "embed": {}, "hr": {}, "img": {}, "input": {},
	"link": {}, "meta": {}, "param": {}, "source": {}, "track": {}, "wbr": {},
}

var bufpool = sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}

var defaultTags = []string{
	"form", "input", "button", "label", "select", "option", "optgroup", "pre", "a", "fieldset", "legend", "textarea",
}

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

func (el *HTMLElement) AddClasses(classes ...string) {
	el.attrs.Classes = append(el.attrs.Classes, classes...)
}

func (el *HTMLElement) RemoveClasses(classes ...string) {
	set := make(map[string]struct{})
	for _, class := range classes {
		set[class] = struct{}{}
	}
	classes = el.attrs.Classes[:0]
	for _, class := range el.attrs.Classes {
		if _, ok := set[class]; ok {
			continue
		}
		classes = append(classes, class)
	}
}

func (el *HTMLElement) SetAttribute(name, value string) {
	if strings.EqualFold(name, "id") {
		el.attrs.ID = value
	} else if strings.EqualFold(name, "class") {
		classes := strings.Split(value, " ")
		el.attrs.Classes = el.attrs.Classes[:0]
		for _, class := range classes {
			if class == "" {
				continue
			}
			el.attrs.Classes = append(el.attrs.Classes, class)
		}
	} else {
		el.attrs.Dict[name] = value
	}
}

func (el *HTMLElement) RemoveAttribute(name string) {
	if strings.EqualFold(name, "id") {
		el.attrs.ID = ""
	} else if strings.EqualFold(name, "class") {
		el.attrs.Classes = el.attrs.Classes[:0]
	} else {
		delete(el.attrs.Dict, name)
	}
}

func (el HTMLElement) Tag() string { return el.attrs.Tag }

func (el HTMLElement) ID() string { return el.attrs.ID }

func (el HTMLElement) WriteHTML(w io.Writer) error {
	err := WriteHTML(w, el.attrs, el.children...)
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

func (txt textValue) WriteHTML(w io.Writer) error {
	for i, value := range txt.values {
		switch value := value.(type) {
		case string:
			if value == "" {
				continue
			}
			if txt.unsafe {
				io.WriteString(w, value)
			} else {
				io.WriteString(w, htmlEscaper(value))
			}
			if strings.TrimSpace(value) == "" {
				continue
			}
			r, _ := utf8.DecodeLastRuneInString(value)
			if i != len(txt.values)-1 && !unicode.IsSpace(r) {
				io.WriteString(w, " ")
			}
		default:
			if txt.unsafe {
				io.WriteString(w, Stringify(value))
			} else {
				io.WriteString(w, htmlEscaper(Stringify(value)))
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

func (l Elements) WriteHTML(w io.Writer) error {
	var err error
	for _, el := range l {
		err = el.WriteHTML(w)
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
	Classes  []string
	Dict     map[string]string
}

func ParseAttributes(selector string, attributes map[string]string) Attributes {
	type State uint8
	const (
		StateNone State = iota
		StateTag
		StateID
		StateClass
		StateAttrName
		StateAttrValue
	)
	attrs := Attributes{Dict: attributes}
	state := StateTag
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
					attrs.Classes = append(attrs.Classes, string(value))
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
			if state == StateAttrName || state == StateAttrValue {
				if _, ok := attrs.Dict[string(name)]; !ok {
					if attrs.Dict == nil {
						attrs.Dict = make(map[string]string)
					}
					switch state {
					case StateAttrName:
						attrs.Dict[string(name)] = Enabled
					case StateAttrValue:
						attrs.Dict[string(name)] = string(value)
					}
				}
			} else {
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
			attrs.Classes = append(attrs.Classes, string(value))
		case StateNone: // do nothing i.e. drop the value
		case StateAttrName, StateAttrValue:
			attrs.ParseErr = fmt.Errorf("unclosed attribute: selector=%s, value: %s", selector, string(value))
			return attrs
		}
		value = value[:0]
	}
	if id, ok := attrs.Dict["id"]; ok {
		delete(attrs.Dict, "id")
		attrs.ID = id
	}
	if class, ok := attrs.Dict["class"]; ok {
		delete(attrs.Dict, "class")
		for _, cls := range strings.Split(class, " ") {
			if cls == "" {
				continue
			}
			attrs.Classes = append(attrs.Classes, cls)
		}
	}
	return attrs
}

func WriteHTML(w io.Writer, attrs Attributes, children ...Element) error {
	var err error
	if attrs.ParseErr != nil {
		return attrs.ParseErr
	}
	if attrs.Tag == "" {
		attrs.Tag = "div"
	}
	escapedTag := htmlNospaceEscaper(attrs.Tag)
	io.WriteString(w, `<`+escapedTag)
	WriteAttributes(w, attrs)
	io.WriteString(w, `>`)
	if _, ok := singletonElements[strings.ToLower(attrs.Tag)]; ok {
		return nil
	}
	if escapedTag == "style" {
		buf := bufpool.Get().(*bytes.Buffer)
		defer func() {
			buf.Reset()
			bufpool.Put(buf)
		}()
		for _, child := range children {
			if child == nil {
				continue
			}
			err = child.WriteHTML(buf)
			if err != nil {
				return err
			}
		}
		io.WriteString(w, cssEscaper(buf.String()))
	} else {
		for _, child := range children {
			if child == nil {
				continue
			}
			err = child.WriteHTML(w)
			if err != nil {
				return err
			}
		}
	}
	io.WriteString(w, `</`+escapedTag+`>`)
	return nil
}

func WriteAttributes(w io.Writer, attrs Attributes) {
	if attrs.ID != "" {
		io.WriteString(w, ` id="`)
		io.WriteString(w, attrEscaper(attrs.ID))
		io.WriteString(w, `"`)
	}
	if len(attrs.Classes) > 0 {
		io.WriteString(w, ` class="`)
		io.WriteString(w, attrEscaper(strings.Join(attrs.Classes, " ")))
		io.WriteString(w, `"`)
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
	var value string
	for _, name := range names {
		value = attrs.Dict[name]
		if name == "" || value == Disabled {
			continue
		}
		io.WriteString(w, ` `)
		io.WriteString(w, htmlNospaceEscaper(name))
		if value == Enabled {
			continue
		}
		io.WriteString(w, `="`)
		if strings.EqualFold(name, "style") {
			io.WriteString(w, cssEscaper(value))
		} else if strings.EqualFold(name, "srcset") {
			io.WriteString(w, srcsetFilterAndEscaper(value))
		} else if t := attrType(name); t == contentTypeURL {
			io.WriteString(w, urlFilter(value))
		} else {
			io.WriteString(w, attrEscaper(value))
		}
		io.WriteString(w, `"`)
	}
}

func Marshal(el Element) (template.HTML, error) {
	buf := bufpool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufpool.Put(buf)
	}()
	err := el.WriteHTML(buf)
	if err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
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
	case nil:
		return "[nil]"
	}
	rv := reflect.ValueOf(v)
	for {
		if rv.Kind() != reflect.Ptr && rv.Kind() != reflect.Interface {
			break
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return "[no value]"
	}
	if rv.Kind() == reflect.Chan {
		return "[channel]"
	}
	if rv.Kind() == reflect.Func {
		return "[function]"
	}
	return fmt.Sprint(v)
}

func NewSanitizer(allowedTags ...string) *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowStyling()
	p.AllowImages()
	p.AllowLists()
	p.AllowTables()
	p.AllowAttrs("inputmode", "hidden").Globally()
	defer p.RequireNoFollowOnLinks(false) // deferred til the last because bluemonday looooves to turn it back on
	for _, tag := range allowedTags {
		p.AllowAttrs(attributesMap[tag]...).OnElements(tag)
	}
	return p
}
