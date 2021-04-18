package hyforms

import (
	"strconv"
	"strings"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/hy"
)

type Input struct {
	form         *Form
	attrs        hy.Attributes
	inputType    string
	name         string
	defaultValue string
}

func (f *Form) Input(inputType string, name string, defaultValue string) *Input {
	f.registerName(name, 1)
	return &Input{form: f, inputType: inputType, name: name, defaultValue: defaultValue}
}

func (f *Form) Text(name string, defaultValue string) *Input {
	f.registerName(name, 1)
	return &Input{form: f, inputType: "text", name: name, defaultValue: defaultValue}
}

func (f *Form) Hidden(name string, defaultValue string) *Input {
	f.registerName(name, 1)
	return &Input{form: f, inputType: "hidden", name: name, defaultValue: defaultValue}
}

func (i *Input) Set(selector string, attributes map[string]string) *Input {
	i.attrs = hy.ParseAttributes(selector, attributes)
	return i
}

func (i *Input) AppendHTML(buf *strings.Builder) error {
	if i.attrs.Dict == nil {
		i.attrs.Dict = make(map[string]string)
	}
	i.attrs.Tag = "input"
	i.attrs.Dict["type"] = i.inputType
	i.attrs.Dict["name"] = i.name
	i.attrs.Dict["value"] = i.defaultValue
	err := hy.AppendHTML(buf, i.attrs, nil)
	if err != nil {
		return erro.Wrap(err)
	}
	return nil
}

func (i *Input) Type() string { return i.inputType }

func (i *Input) ID() string { return i.attrs.ID }

func (i *Input) Name() string { return i.name }

func (i *Input) DefaultValue() string { return i.defaultValue }

func (i *Input) Value() string {
	if i.form.mode != FormModeUnmarshal {
		return ""
	}
	if len(i.form.request.Form[i.name]) == 0 {
		return ""
	}
	return i.form.request.Form[i.name][0]
}

func (i *Input) Int(validators ...Validator) (num int, err error) {
	if i.form.mode != FormModeUnmarshal {
		return 0, nil
	}
	value := i.form.request.FormValue(i.name)
	num, err = strconv.Atoi(value)
	if err != nil {
		return 0, erro.Wrap(err)
	}
	validateInput(i.form, i.name, num, validators)
	return num, nil
}

func (i *Input) Float64(validators ...Validator) (num float64, err error) {
	if i.form.mode != FormModeUnmarshal {
		return 0, nil
	}
	value := i.form.request.FormValue(i.name)
	num, err = strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, erro.Wrap(err)
	}
	validateInput(i.form, i.name, num, validators)
	return num, nil
}

func (i *Input) Validate(validators ...Validator) *Input {
	var value interface{}
	if len(i.form.request.Form[i.name]) > 0 {
		value = i.form.request.Form[i.name][0]
	}
	validateInput(i.form, i.name, value, validators)
	return i
}

func (i *Input) ErrMsgs() []string {
	return i.form.inputErrMsgs[i.name]
}

type ToggledInput struct {
	form      *Form
	attrs     hy.Attributes
	inputType string
	name      string
	value     string
	checked   bool
}

func (f *Form) Checkbox(name string, value string, checked bool) *ToggledInput {
	f.registerName(name, 1)
	return &ToggledInput{form: f, inputType: "checkbox", name: name, value: value, checked: checked}
}

func (f *Form) Radio(name string, value string, checked bool) *ToggledInput {
	f.registerName(name, 1)
	return &ToggledInput{form: f, inputType: "radio", name: name, value: value, checked: checked}
}

func (i *ToggledInput) Set(selector string, attributes map[string]string) *ToggledInput {
	i.attrs = hy.ParseAttributes(selector, attributes)
	return i
}

func (i *ToggledInput) AppendHTML(buf *strings.Builder) error {
	if i.attrs.Dict == nil {
		i.attrs.Dict = make(map[string]string)
	}
	i.attrs.Tag = "input"
	i.attrs.Dict["type"] = i.inputType
	i.attrs.Dict["name"] = i.name
	if i.value != "" {
		i.attrs.Dict["value"] = i.value
	}
	if i.checked {
		i.attrs.Dict["checked"] = hy.Enabled
	} else {
		i.attrs.Dict["checked"] = hy.Disabled
	}
	err := hy.AppendHTML(buf, i.attrs, nil)
	if err != nil {
		return erro.Wrap(err)
	}
	return nil
}

func (i *ToggledInput) Type() string { return i.inputType }

func (i *ToggledInput) ID() string { return i.attrs.ID }

func (i *ToggledInput) Name() string { return i.name }

func (i *ToggledInput) Value() string { return i.value }

func (i *ToggledInput) ErrMsgs() []string {
	return i.form.inputErrMsgs[i.name]
}

func (i *ToggledInput) Check(b bool) *ToggledInput {
	i.checked = b
	return i
}

func (i *ToggledInput) Checked() bool {
	if i.form.mode != FormModeUnmarshal {
		return false
	}
	values, ok := i.form.request.Form[i.name]
	if !ok || len(values) == 0 {
		return false
	}
	for _, v := range values {
		if i.value == "" && v == "on" {
			return true
		}
		if i.value != "" && v == i.value {
			return true
		}
	}
	return false
}

type ToggledInputs struct {
	form      *Form
	inputType string
	name      string
	options   []string
}

func (f *Form) Checkboxes(name string, options []string) *ToggledInputs {
	f.registerName(name, 1)
	return &ToggledInputs{form: f, inputType: "checkbox", name: name, options: options}
}

func (f *Form) Radios(name string, options []string) *ToggledInputs {
	f.registerName(name, 1)
	return &ToggledInputs{form: f, inputType: "radio", name: name, options: options}
}

func (i *ToggledInputs) Inputs() []*ToggledInput {
	var inputs []*ToggledInput
	for _, option := range i.options {
		inputs = append(inputs, &ToggledInput{form: i.form, inputType: i.inputType, name: i.name, value: option})
	}
	return inputs
}

func (i *ToggledInputs) Options() []string {
	return i.options
}

func (i *ToggledInputs) Value() string {
	if i.form.mode != FormModeUnmarshal {
		return ""
	}
	return i.form.request.FormValue(i.name)
}

func (i *ToggledInputs) Values() []string {
	if i.form.mode != FormModeUnmarshal {
		return nil
	}
	return i.form.request.Form[i.name]
}

type Options []Option

type Option struct {
	Value      string
	Display    string
	Disabled   bool
	Selected   bool
	Selector   string
	Attributes map[string]string
	Optgroup   string
	Options    Options
}

func (o Option) AppendHTML(buf *strings.Builder) error {
	attrs := hy.ParseAttributes(o.Selector, o.Attributes)
	attrs.Tag = "option"
	attrs.Dict["value"] = o.Value
	if o.Disabled {
		attrs.Dict["disabled"] = hy.Enabled
	}
	if o.Selected {
		attrs.Dict["selected"] = hy.Enabled
	}
	err := hy.AppendHTML(buf, attrs, []hy.Element{hy.Txt(o.Display)})
	if err != nil {
		return erro.Wrap(err)
	}
	return nil
}

type SelectInput struct {
	form    *Form
	attrs   hy.Attributes
	name    string
	Options Options
}

func (f *Form) Select(name string, options []Option) *SelectInput {
	f.registerName(name, 1)
	return &SelectInput{form: f, name: name, Options: options}
}

func (i *SelectInput) Set(selector string, attributes map[string]string) *SelectInput {
	i.attrs = hy.ParseAttributes(selector, attributes)
	return i
}

func (i *SelectInput) AppendHTML(buf *strings.Builder) error {
	if i.attrs.Dict == nil {
		i.attrs.Dict = make(map[string]string)
	}
	if i.attrs.ParseErr != nil {
		return erro.Wrap(i.attrs.ParseErr)
	}
	buf.WriteString(`<select`)
	if i.attrs.ID != "" {
		buf.WriteString(` id="` + i.attrs.ID + `"`)
	}
	if i.attrs.Class != "" {
		buf.WriteString(` class="` + i.attrs.Class + `"`)
	}
	if i.name != "" {
		i.attrs.Dict["name"] = i.name
	}
	for name, value := range i.attrs.Dict {
		switch value {
		case hy.Enabled:
			buf.WriteString(` ` + name)
		case hy.Disabled, "id", "class":
			continue
		default:
			buf.WriteString(` ` + name + `="` + value + `"`)
		}
	}
	buf.WriteString(`>`)
	var err error
	for _, opt := range i.Options {
		switch opt.Optgroup {
		case "":
			opt.AppendHTML(buf)
		default:
			attrs := hy.ParseAttributes(opt.Selector, opt.Attributes)
			attrs.Tag = "optgroup"
			attrs.Dict["label"] = opt.Optgroup
			if opt.Disabled {
				attrs.Dict["disabled"] = hy.Enabled
			}
			if opt.Selected {
				attrs.Dict["selected"] = hy.Enabled
			}
			var children []hy.Element
			for _, option := range opt.Options {
				if len(option.Options) > 0 {
					continue
				}
				children = append(children, option)
			}
			err = hy.AppendHTML(buf, attrs, children)
			if err != nil {
				return erro.Wrap(err)
			}
		}
	}
	buf.WriteString(`</select>`)
	return nil
}

func (i *SelectInput) Values() []string {
	if i.form.mode != FormModeUnmarshal {
		return nil
	}
	return i.form.request.Form[i.name]
}
