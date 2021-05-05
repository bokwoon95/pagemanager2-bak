// Package hy implements hyperscript in Go.
package hy

import (
	"bytes"
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
)

const (
	Enabled  = "\x00"
	Disabled = "\x01"
)

type Element interface {
	WriteHTML(buf *bytes.Buffer, allow func(tag, attrName, attrValue string) bool) error
}

// https://developer.mozilla.org/en-US/docs/Glossary/Empty_element
var singletonElements = map[string]struct{}{
	"area": {}, "base": {}, "br": {}, "col": {}, "embed": {}, "hr": {}, "img": {}, "input": {},
	"link": {}, "meta": {}, "param": {}, "source": {}, "track": {}, "wbr": {},
}

var bufpool = sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}

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
	excluded := make(map[string]struct{})
	for _, class := range classes {
		excluded[class] = struct{}{}
	}
	classes = el.attrs.Classes[:0]
	for _, class := range el.attrs.Classes {
		if _, ok := excluded[class]; ok {
			continue
		}
		classes = append(classes, class)
	}
	el.attrs.Classes = el.attrs.Classes[:len(classes)]
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
		if el.attrs.Dict == nil {
			el.attrs.Dict = make(map[string]string)
		}
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

func (el HTMLElement) WriteHTML(buf *bytes.Buffer, allow func(tag, attrName, attrValue string) bool) error {
	err := WriteHTML(buf, el.attrs, el.children, allow)
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

func (txt textValue) WriteHTML(buf *bytes.Buffer, allow func(tag, attrName, attrValue string) bool) error {
	last := len(txt.values) - 1
	for i, value := range txt.values {
		switch value := value.(type) {
		case string:
			if value == "" {
				continue
			}
			if txt.unsafe {
				buf.WriteString(value)
			} else {
				escapeRunes(buf, htmlReplacementTableV2, value)
			}
			if strings.TrimSpace(value) == "" {
				continue
			}
			r, _ := utf8.DecodeLastRuneInString(value)
			if i != last && !unicode.IsSpace(r) {
				buf.WriteByte(' ')
			}
		default:
			if txt.unsafe {
				buf.WriteString(Stringify(value))
			} else {
				escapeRunes(buf, htmlReplacementTableV2, Stringify(value))
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

func (l Elements) WriteHTML(buf *bytes.Buffer, allow func(tag, attrName, attrValue string) bool) error {
	var err error
	for _, el := range l {
		if el == nil {
			continue
		}
		err = el.WriteHTML(buf, allow)
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
		for _, c := range strings.Split(class, " ") {
			if c == "" {
				continue
			}
			attrs.Classes = append(attrs.Classes, c)
		}
	}
	return attrs
}

func WriteHTML(buf *bytes.Buffer, attrs Attributes, children []Element, allow func(tag, attrName, attrValue string) bool) error {
	if allow == nil {
		allow = Allow
	}
	var err error
	if attrs.ParseErr != nil {
		return attrs.ParseErr
	}
	if attrs.Tag == "" {
		attrs.Tag = "div"
	}
	if !allow(attrs.Tag, "", "") {
		return nil
	}
	buf.WriteString(`<` + attrs.Tag)
	WriteAttributes(buf, attrs, allow)
	buf.WriteString(`>`)
	if _, ok := singletonElements[strings.ToLower(attrs.Tag)]; ok {
		return nil
	}
	for _, child := range children {
		if child == nil {
			continue
		}
		err = child.WriteHTML(buf, allow)
		if err != nil {
			return err
		}
	}
	buf.WriteString(`</` + attrs.Tag + `>`)
	return nil
}

func WriteAttributes(buf *bytes.Buffer, attrs Attributes, allow func(tag, attrName, attrValue string) bool) {
	if allow == nil {
		allow = Allow
	}
	if attrs.ID != "" {
		buf.WriteString(` id="`)
		escapeRunes(buf, htmlAttrReplacementTable, attrs.ID)
		buf.WriteString(`"`)
	}
	if len(attrs.Classes) > 0 {
		buf.WriteString(` class="`)
		escapeRunes(buf, htmlAttrReplacementTable, strings.Join(attrs.Classes, " "))
		buf.WriteString(`"`)
	}
	var names []string
	for name := range attrs.Dict {
		if strings.EqualFold(name, "id") || strings.EqualFold(name, "class") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var value string
	for _, name := range names {
		value = attrs.Dict[name]
		if name == "" || value == Disabled {
			continue
		}
		if !allow(attrs.Tag, name, value) {
			continue
		}
		buf.WriteString(` `)
		escapeRunes(buf, htmlAttrReplacementTable, name)
		if value == Enabled {
			continue
		}
		buf.WriteString(`="`)
		if isURLAttr(name) {
			escapeURL(buf, value)
		} else if strings.EqualFold(name, "srcset") {
			escapeSrcset(buf, value)
		} else {
			escapeRunes(buf, htmlAttrReplacementTable, value)
		}
		buf.WriteString(`"`)
	}
}

func Marshal(el Element) (template.HTML, error) {
	return CustomMarshal(el, Allow)
}

func CustomMarshal(el Element, allow func(tag, attrName, attrValue string) bool) (template.HTML, error) {
	if allow == nil {
		allow = Allow
	}
	buf := bufpool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufpool.Put(buf)
	}()
	if el == nil {
		return "", nil
	}
	err := el.WriteHTML(buf, allow)
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
