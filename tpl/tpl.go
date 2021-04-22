package tpl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"
)

var bufpool = sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}

type RenderOption func(*Renderer)

type Renderer struct {
	fs                   fs.FS
	files                []string
	basefile             string
	funcMap              map[string]interface{}
	templateOption       []string
	cacheGet             func(w http.ResponseWriter, r *http.Request, files []string) (*template.Template, error)
	cacheSet             func(w http.ResponseWriter, r *http.Request, files []string, t *template.Template) error
	shouldJSON           func(w http.ResponseWriter, r *http.Request, data interface{}) (bool, error)
	jsonMarshaller       func(w http.ResponseWriter, r *http.Request, data interface{}) ([]byte, error)
	alwaysParseTemplates bool
	rendering            bool
	redeclaredFuncMap    bool
	redeclaredFS         bool
}

func New(fsys fs.FS, opts ...RenderOption) Renderer {
	rdr := Renderer{fs: fsys, funcMap: make(map[string]interface{})}
	for _, opt := range opts {
		opt(&rdr)
	}
	return rdr
}

func Render(w http.ResponseWriter, r *http.Request, data interface{}, opts ...RenderOption) error {
	return New(nil).Render(w, r, data, opts...)
}

func (rdr Renderer) Render(w http.ResponseWriter, r *http.Request, data interface{}, opts ...RenderOption) error {
	rdr.rendering = true
	for _, opt := range opts {
		opt(&rdr)
	}
	var shouldJSON bool
	var err error
	if rdr.shouldJSON != nil {
		shouldJSON, err = rdr.shouldJSON(w, r, data)
		if err != nil {
			return fmt.Errorf("tpl: error when calling shouldJSON: %w", err)
		}
	}
	if shouldJSON && !rdr.alwaysParseTemplates {
		return rdr.marshalJSON(w, r, data)
	}
	t, err := rdr.parseTemplate(w, r)
	if err != nil {
		return err
	}
	if shouldJSON {
		return rdr.marshalJSON(w, r, data)
	}
	buf := bufpool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufpool.Put(buf)
	}()
	err = t.ExecuteTemplate(buf, rdr.basefile, data)
	if err != nil {
		return fmt.Errorf("tpl: failed to execute template: %w", err)
	}
	buf.WriteTo(w)
	return nil
}

func (rdr Renderer) marshalJSON(w http.ResponseWriter, r *http.Request, data interface{}) error {
	var b []byte
	var err error
	if rdr.jsonMarshaller != nil {
		b, err = rdr.jsonMarshaller(w, r, data)
	} else {
		b, err = json.Marshal(data)
	}
	if err != nil {
		return fmt.Errorf("tpl: failed to marshal json: %w", err)
	}
	w.Write(b)
	return nil
}

func (rdr Renderer) parseTemplate(w http.ResponseWriter, r *http.Request) (*template.Template, error) {
	if rdr.basefile == "" {
		return nil, fmt.Errorf("no files provided")
	}
	var t *template.Template
	var err error
	doNotCache := rdr.redeclaredFuncMap || rdr.redeclaredFS
	allFiles := make([]string, len(rdr.files)+1)
	allFiles[0] = rdr.basefile
	copy(allFiles[1:], rdr.files)
	if rdr.cacheGet != nil && !doNotCache {
		t, err = rdr.cacheGet(w, r, allFiles)
		if err != nil {
			return t, fmt.Errorf("tpl: error when calling cacheGet: %w", err)
		}
		return t, nil
	}
	if rdr.fs == nil {
		return nil, fmt.Errorf("no FS provided")
	}
	var b []byte
	t = template.New(rdr.basefile)
	if len(rdr.templateOption) > 0 {
		t.Option(rdr.templateOption...)
	}
	if len(rdr.funcMap) > 0 {
		t.Funcs(rdr.funcMap)
	}
	b, err = fs.ReadFile(rdr.fs, rdr.basefile)
	if err != nil {
		return nil, fmt.Errorf("error when reading file %s: %w", rdr.basefile, err)
	}
	t, err = t.Parse(string(b))
	if err != nil {
		return nil, fmt.Errorf("error when parsing file %s: %w", rdr.basefile, err)
	}
	for _, file := range rdr.files {
		b, err = fs.ReadFile(rdr.fs, file)
		if err != nil {
			return nil, fmt.Errorf("error when reading file %s: %w", file, err)
		}
		t, err = t.New(file).Parse(string(b))
		if err != nil {
			return nil, fmt.Errorf("error when parsing file %s: %w", file, err)
		}
	}
	if rdr.cacheSet != nil && !doNotCache {
		err = rdr.cacheSet(w, r, allFiles, t)
		if err != nil {
			return t, fmt.Errorf("error when calling cacheSet: %w", err)
		}
	}
	return t, nil
}

func FSFiles(fsys fs.FS, files ...string) RenderOption {
	return func(rdr *Renderer) {
		FS(fsys)(rdr)
		Files(files...)(rdr)
	}
}

func FS(fsys fs.FS) RenderOption {
	return func(rdr *Renderer) {
		if rdr.rendering {
			rdr.redeclaredFS = true
		}
		rdr.fs = fsys
	}
}

func Files(files ...string) RenderOption {
	return func(rdr *Renderer) {
		if rdr.rendering {
			if len(files) == 0 {
				return
			}
			rdr.basefile = files[0]
			rdr.files = append(rdr.files, files[1:]...)
		} else {
			rdr.files = append(rdr.files, files...)
		}
	}
}

func FuncMap(funcMap map[string]interface{}) RenderOption {
	return func(rdr *Renderer) {
		if rdr.rendering {
			rdr.redeclaredFuncMap = true
		}
		for name, fn := range funcMap {
			rdr.funcMap[name] = fn
		}
	}
}

func TemplateOption(opt ...string) RenderOption {
	return func(rdr *Renderer) { rdr.templateOption = append(rdr.templateOption, opt...) }
}

func DefaultCache() RenderOption {
	cache := make(map[string]*template.Template)
	cacheGet := func(_ http.ResponseWriter, _ *http.Request, files []string) (*template.Template, error) {
		fullname := strings.Join(files, "\n")
		return cache[fullname], nil
	}
	cacheSet := func(_ http.ResponseWriter, _ *http.Request, files []string, t *template.Template) error {
		fullname := strings.Join(files, "\n")
		cache[fullname] = t
		return nil
	}
	return func(rdr *Renderer) {
		rdr.cacheGet = cacheGet
		rdr.cacheSet = cacheSet
	}
}

func CacheGet(cacheGet func(w http.ResponseWriter, r *http.Request, files []string) (*template.Template, error)) RenderOption {
	return func(rdr *Renderer) { rdr.cacheGet = cacheGet }
}

func CacheSet(cacheSet func(w http.ResponseWriter, r *http.Request, files []string, t *template.Template) error) RenderOption {
	return func(rdr *Renderer) { rdr.cacheSet = cacheSet }
}

func ShouldJSON(shouldJSON func(w http.ResponseWriter, r *http.Request, data interface{}) (bool, error)) RenderOption {
	return func(rdr *Renderer) { rdr.shouldJSON = shouldJSON }
}

func JSONMarshaller(jsonMarshaller func(w http.ResponseWriter, r *http.Request, data interface{}) ([]byte, error)) RenderOption {
	return func(rdr *Renderer) { rdr.jsonMarshaller = jsonMarshaller }
}

func AlwaysParseTemplates(alwaysParseTemplates bool) RenderOption {
	return func(rdr *Renderer) { rdr.alwaysParseTemplates = alwaysParseTemplates }
}
