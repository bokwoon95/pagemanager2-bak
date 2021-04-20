package hyforms

import (
	"html/template"
	"testing"

	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/testutil"
)

func Test_SelectInput(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		is := testutil.New(t)
		form := &Form{
			inputNames:   make(map[string]struct{}),
			inputErrMsgs: make(map[string][]string),
		}
		sel := form.Select("my-select", Options{
			{Value: "0", Display: "Option 0"},
			{Optgroup: "Group 1", Options: Options{
				{Value: "1.1", Display: "Option 1.1"},
			}},
			{Optgroup: "Group 2", Options: Options{
				{Value: "2.1", Display: "Option 2.1"},
				{Value: "2.2", Display: "Option 2.2"},
			}},
			{Optgroup: "Group 3", Options: Options{
				{Value: "3.1", Display: "Option 3.1"},
				{Value: "3.2", Display: "Option 3.2"},
				{Value: "3.3", Display: "Option 3.3"},
			}},
		})
		sel.Set("#my-select", nil)
		got, err := hy.Marshal(nil, sel)
		is.NoErr(err)
		want := `<select id="my-select">` +
			`<option value="0">Option 0</option>` +
			`<optgroup label="Group 1">` +
			`<option value="1.1">Option 1.1</option>` +
			`</optgroup>` +
			`<optgroup label="Group 2">` +
			`<option value="2.1">Option 2.1</option>` +
			`<option value="2.2">Option 2.2</option>` +
			`</optgroup>` +
			`<optgroup label="Group 3">` +
			`<option value="3.1">Option 3.1</option>` +
			`<option value="3.2">Option 3.2</option>` +
			`<option value="3.3">Option 3.3</option>` +
			`</optgroup>` +
			`</select>`
		is.Equal(template.HTML(want), got)
	})
}
