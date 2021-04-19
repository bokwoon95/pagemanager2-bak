package pagemanager

import (
	"fmt"
	"html/template"
	"io"
	"net/http"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/hy"
)

type errorPageData struct {
	Title  string
	Header template.HTML
	ErrMsg template.HTML
}

func (pm *PageManager) InternalServerError(w http.ResponseWriter, r *http.Request, serverErr error) {
	var err error
	data := errorPageData{
		Title:  "500 Internal Server Error",
		Header: "500 Internal Server Error",
	}
	var els hy.Elements
	els.Append("p.f4", nil, hy.Txt("Something went wrong, here is the error trace (read top down)"))
	els.Append("p.f5", nil, hy.Txt("URL: "+LocaleURL(r, "")))
	els.Append("pre", nil, hy.Txt(erro.Sdump(serverErr)))
	data.ErrMsg, err = hy.MarshalElement(nil, els)
	if err != nil {
		io.WriteString(w, fmt.Errorf("%w: %s", serverErr, err).Error())
		return
	}
	err = pm.executeTemplates(w, data, pagemanagerFS, "error_page.html")
	if err != nil {
		io.WriteString(w, fmt.Errorf("%w: %s", serverErr, err).Error())
		return
	}

}

func (pm *PageManager) Unauthorized(w http.ResponseWriter, r *http.Request) {
	var err error
	data := errorPageData{
		Title:  "401 Unauthorized",
		Header: "401 Unauthorized",
	}
	err = pm.executeTemplates(w, data, pagemanagerFS, "error_page.html")
	if err != nil {
		io.WriteString(w, fmt.Errorf("401 Unauthorized: %s", err).Error())
		return
	}
}

func (pm *PageManager) Forbidden(w http.ResponseWriter, r *http.Request) {
	var err error
	data := errorPageData{
		Title:  "403 Forbidden",
		Header: "403 Forbidden",
	}
	err = pm.executeTemplates(w, data, pagemanagerFS, "error_page.html")
	if err != nil {
		io.WriteString(w, fmt.Errorf("403 Forbidden: %s", err).Error())
		return
	}
}
