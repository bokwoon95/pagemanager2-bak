package hy

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bokwoon95/pagemanager/testutil"
)

func Test_ParseAttributes(t *testing.T) {
	assertOK := func(t *testing.T, selector string, attributes map[string]string, want Attributes) {
		is := testutil.New(t, testutil.Parallel, testutil.FailFast)
		got := ParseAttributes(selector, attributes)
		is.NoErr(got.ParseErr)
		is.Equal(want, got)
	}
	t.Run("empty", func(t *testing.T) {
		selector := "div"
		assertOK(t, selector, map[string]string{}, Attributes{
			Tag:  "div",
			Dict: map[string]string{},
		})
	})
	t.Run("tag only", func(t *testing.T) {
		selector := "div"
		assertOK(t, selector, map[string]string{}, Attributes{
			Tag:  "div",
			Dict: map[string]string{},
		})
	})
	t.Run("selector tags, id, classes and attributes", func(t *testing.T) {
		selector := "p#id1.class1.class2.class3#id2[attr1=val1][attr2=val2][attr3=val3][attr4]"
		assertOK(t, selector, map[string]string{}, Attributes{
			Tag:   "p",
			ID:    "id2",
			Class: []string{"class1", "class2", "class3"},
			Dict: map[string]string{
				"attr1": "val1",
				"attr2": "val2",
				"attr3": "val3",
				"attr4": Enabled,
			},
		})
	})
	t.Run("attributes overwrite selector", func(t *testing.T) {
		selector := "p#id1.class1.class2.class3#id2[attr1=val1][attr2=val2][attr3=val3][attr4]"
		attributes := map[string]string{
			"id":    "id3",
			"class": "class4 class5 class6",
			"attr1": `value-1`,
			"attr2": "value-2",
			"attr3": "value-3",
			"attr4": Disabled,
		}
		assertOK(t, selector, attributes, Attributes{
			Tag:   "p",
			ID:    "id3",
			Class: []string{"class1", "class2", "class3", "class4", "class5", "class6"},
			Dict: map[string]string{
				"attr1": "value-1",
				"attr2": "value-2",
				"attr3": "value-3",
				"attr4": Disabled,
			},
		})
	})
}

func Test_XSS(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		is := testutil.New(t)
		div := H("", Attr{"id": `no-op" class="http://malicious.com"`})
		out, err := Marshal(div)
		is.NoErr(err)
		fmt.Println(out)
	})
	t.Run("basic", func(t *testing.T) {
		is := testutil.New(t)
		div := H("div", nil, Txt("<script>alert('xss')</script>"))
		out, err := Marshal(div)
		is.NoErr(err)
		fmt.Println(out)
	})
	t.Run("href", func(t *testing.T) {
		is := testutil.New(t)
		div := H("a", Attr{"href": "/users"}, Txt("users"))
		out, err := Marshal(div)
		is.NoErr(err)
		fmt.Println(out)
	})
	t.Run("with template", func(t *testing.T) {
		is := testutil.New(t)
		tmpl, err := template.New("").Parse(`{{ . }}`)
		is.NoErr(err)
		payload, err := Marshal(H("script", nil, Txt(`Set.constructor`+"`"+`alert\x28document.domain\x29`)))
		is.NoErr(err)
		buf := &bytes.Buffer{}
		err = tmpl.Execute(buf, payload)
		is.NoErr(err)
		fmt.Println(buf.String())
	})
}

type tableRows []struct {
	Name         string
	Breed        string
	Age          int
	Owner        string
	EatingHabits string
}

var tdata = tableRows{
	{"Knocky", "Jack Russell", 16, "Mother-in-law", "Eats everyone's leftovers"},
	{"Flor", "Poodle", 9, "Me", "Nibbles at food"},
	{"Ella", "Streetdog", 10, "Me", "Hearty eater"},
	{"Juan", "Cocker Spaniel", 5, "Sister-in-law", "Will eat til he explodes"},
}

func BenchmarkHTML(b *testing.B) {
	_, currentfile, _, _ := runtime.Caller(0)
	currentdir := os.DirFS(filepath.Dir(currentfile))
	t, err := template.ParseFS(currentdir, "template_html.html")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	buf := &bytes.Buffer{}
	for i := 0; i < b.N; i++ {
		t.Execute(buf, tdata)
		_ = buf.String()
		buf.Reset()
	}
}

func (rs tableRows) TableRows() (template.HTML, error) {
	var els Elements
	for _, r := range rs {
		els.Append("tr", nil,
			H("td", nil, Txt(r.Name)),
			H("td", nil, Txt(r.Breed)),
			H("td", nil, Txt(r.Age)),
			H("td", nil, Txt(r.Owner)),
			H("td", nil, Txt(r.EatingHabits)),
		)
	}
	return Marshal(els)
}

func BenchmarkHy(b *testing.B) {
	_, currentfile, _, _ := runtime.Caller(0)
	currentdir := os.DirFS(filepath.Dir(currentfile))
	t, err := template.ParseFS(currentdir, "template_hy.html")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	buf := &bytes.Buffer{}
	for i := 0; i < b.N; i++ {
		t.Execute(buf, tdata)
		_ = buf.String()
		buf.Reset()
	}
}
