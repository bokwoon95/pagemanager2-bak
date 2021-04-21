package pagemanager

import (
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"net/http"
	"sort"
	"strings"

	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/hy"
)

type InlinedJSElement struct {
	w       http.ResponseWriter
	fsys    fs.FS
	files   []string
	skipCSP bool
}

func InlinedJS(w http.ResponseWriter, fsys fs.FS, files []string) InlinedJSElement {
	return InlinedJSElement{w: w, fsys: fsys, files: files}
}

func (el InlinedJSElement) AppendHTML(buf *strings.Builder) error {
	b64HashSet := make(map[string]struct{})
	for _, file := range el.files {
		data, err := fs.ReadFile(el.fsys, file)
		if err != nil {
			return erro.Wrap(err)
		}
		hash := sha256.Sum256(data)
		b64Hash := base64.StdEncoding.EncodeToString(hash[:])
		b64HashSet[`'sha256-`+b64Hash+`'`] = struct{}{}
		attrs := hy.Attributes{Tag: "script"}
		err = hy.AppendHTML(buf, attrs, []hy.Element{hy.UnsafeTxt(data)})
		if err != nil {
			return erro.Wrap(err)
		}
	}
	if el.skipCSP {
		return nil
	}
	b64HashList := make([]string, len(b64HashSet))
	i := 0
	for b64Hash := range b64HashSet {
		b64HashList[i] = b64Hash
		i++
	}
	sort.Strings(b64HashList)
	err := appendCSP(el.w, "script-src", strings.Join(b64HashList, " "))
	if err != nil {
		return erro.Wrap(err)
	}
	return nil
}
