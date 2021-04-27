package pagemanager

import (
	"fmt"
	"html/template"
	"io"
	"net/http"

	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
	"github.com/bokwoon95/pagemanager/tpl"
)

type errorPageData struct {
	Title  string
	Header template.HTML
	ErrMsg template.HTML
}

func (pm *PageManager) InternalServerError(w http.ResponseWriter, r *http.Request, serverErr error) {
	var err error
	w.WriteHeader(http.StatusInternalServerError)
	data := errorPageData{
		Title:  "500 Internal Server Error",
		Header: "500 Internal Server Error",
	}
	data.ErrMsg, err = hy.Marshal(hy.Elements{
		hy.H("p.f4", nil, hy.Txt("Something went wrong, here is the error trace (read top down)")),
		hy.H("p.f5", nil, hy.Txt("URL:", LocaleURL(r, ""))),
		hy.H("pre.white-space-prewrap.word-wrap", nil, hy.Txt(erro.Sdump(serverErr))),
	})
	if err != nil {
		io.WriteString(w, fmt.Errorf("%w: %s", serverErr, err).Error())
		return
	}
	err = pm.tpl.Render(w, r, data, tpl.NewFiles("error_page.html"))
	if err != nil {
		io.WriteString(w, fmt.Errorf("%w: %s", serverErr, err).Error())
		return
	}
}

func (pm *PageManager) Unauthorized(w http.ResponseWriter, r *http.Request) {
	var err error
	w.WriteHeader(http.StatusUnauthorized)
	data := errorPageData{
		Title:  "401 Unauthorized",
		Header: "401 Unauthorized",
	}
	els := hy.Elements{
		hy.H("p", nil, hy.Txt("You need to be an authorized user.")),
	}
	USERS := tables.NEW_USERS(r.Context(), "u")
	exists, _ := sq.Exists(pm.dataDB, sq.SQLite.From(USERS).Where(USERS.USER_ID.NeInt(1)))
	if exists {
		els.AppendElements(hy.H("a", hy.Attr{"href": "/pm-login"}, hy.Txt("Log In")), hy.Txt(", or "))
	}
	els.Append("p", nil, hy.H("a", hy.Attr{"href": "/pm-superadmin-login"}, hy.Txt("Log In as Superadmin")), hy.Txt("."))
	els.Append("p", nil, hy.H("a", hy.Attr{"href": LocaleURL(r, "/")}, hy.Txt("Go Home")), hy.Txt("."))
	data.ErrMsg, err = hy.Marshal(els)
	if err != nil {
		io.WriteString(w, fmt.Errorf("403 Forbidden: %s", err).Error())
		return
	}
	err = pm.tpl.Render(w, r, data, tpl.NewFiles("error_page.html"))
	if err != nil {
		io.WriteString(w, fmt.Errorf("403 Forbidden: %s", err).Error())
		return
	}
}

func (pm *PageManager) Forbidden(w http.ResponseWriter, r *http.Request) {
	var err error
	w.WriteHeader(http.StatusForbidden)
	data := errorPageData{
		Title:  "403 Forbidden",
		Header: "403 Forbidden",
	}
	data.ErrMsg, _ = hy.Marshal(hy.Elements{
		hy.H("p", nil, hy.Txt("You are not authorized to do this action.")),
		hy.H("p", nil, hy.H("a", hy.Attr{"href": LocaleURL(r, "/")}, hy.Txt("Go Home")), hy.Txt(".")),
	})
	err = pm.tpl.Render(w, r, data, tpl.NewFiles("error_page.html"))
	if err != nil {
		io.WriteString(w, fmt.Errorf("401 Unauthorized: %s", err).Error())
		return
	}
}
