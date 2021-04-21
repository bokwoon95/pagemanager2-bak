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
)

// https://developer.mozilla.org/en-US/docs/Glossary/Empty_element
var singletonElements = map[string]struct{}{
	"AREA": {}, "BASE": {}, "BR": {}, "COL": {}, "EMBED": {}, "HR": {}, "IMG": {}, "INPUT": {},
	"LINK": {}, "META": {}, "PARAM": {}, "SOURCE": {}, "TRACK": {}, "WBR": {},
}

var bufpool = sync.Pool{
	New: func() interface{} { return &strings.Builder{} },
}

const (
	Enabled  = "\x00"
	Disabled = "\x01"
)

type Element interface {
	AppendHTML(*strings.Builder) error
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
}

func Txt(a ...interface{}) Element {
	return textValue{values: a}
}

func (txt textValue) AppendHTML(buf *strings.Builder) error {
	for _, value := range txt.values {
		switch value := value.(type) {
		case string:
			if value == "" {
				continue
			}
			buf.WriteString(value)
			if strings.TrimSpace(value) == "" {
				continue
			}
			if r, _ := utf8.DecodeLastRuneInString(value); !unicode.IsSpace(r) {
				buf.WriteByte(' ')
			}
		default:
			buf.WriteString(Stringify(value))
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
				attrs.Class += " " + value
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
		buf.WriteString(`<` + attrs.Tag)
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
		buf.WriteString("</" + attrs.Tag + ">")
	}
	return nil
}

func AppendAttributes(buf *strings.Builder, attrs Attributes) {
	if attrs.ID != "" {
		buf.WriteString(` id="` + attrs.ID + `"`)
	}
	if attrs.Class != "" {
		buf.WriteString(` class="` + attrs.Class + `"`)
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
		value := attrs.Dict[name]
		switch value {
		case Enabled:
			buf.WriteString(` ` + name)
		case Disabled:
			continue
		default:
			buf.WriteString(` ` + name + `="` + value + `"`)
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
		s = defaultSanitizer
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
