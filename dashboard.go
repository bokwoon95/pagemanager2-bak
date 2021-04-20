package pagemanager

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/hyforms"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
)

type dashboardData struct {
	Routes []Route
}

func (pm *PageManager) dashboard(w http.ResponseWriter, r *http.Request) {
	type templateData struct {
		Routes template.HTML
	}
	r.ParseForm()
	var err error
	switch r.Method {
	case "GET":
		user := pm.getUser(w, r)
		switch {
		case !user.Valid:
			_ = hyforms.SetCookieValue(w, cookieLoginRedirect, LocaleURL(r, r.URL.Path), nil)
			pm.RedirectToLogin(w, r)
			return
		case !user.HasPagePerms(PagePermsRead):
			pm.Forbidden(w, r)
			return
		}
		data := dashboardData{}
		p := tables.NEW_PAGES(r.Context(), "r")
		_, err = sq.Fetch(pm.dataDB, sq.SQLite.From(p).OrderBy(p.URL), func(row *sq.Row) error {
			var route Route
			if err := route.RowMapper(p)(row); err != nil {
				return erro.Wrap(err)
			}
			return row.Accumulate(func() error {
				data.Routes = append(data.Routes, route)
				return nil
			})
		})
		if len(r.Form[queryparamJSON]) > 0 {
			b, err := json.Marshal(data)
			if err != nil {
				pm.InternalServerError(w, r, erro.Wrap(err))
			} else {
				w.Write(b)
			}
			return
		}
		var els hy.Elements
		els.Append("div.mv2", nil, hy.H("a", hy.Attr{"href": URLCreatePage}, hy.Txt("create")))
		for _, route := range data.Routes {
			div := hy.H("div.mv2", nil)
			div.Append("div", nil, hy.Txt("URL: "), hy.Txt(route.URL.String))
			switch {
			case !route.URL.Valid:
				continue
			case route.Disabled.Valid:
				div.Append("div", nil,
					hy.Txt("Disabled: "),
					hy.Txt(route.Disabled.Bool),
					hy.H("a", hy.Attr{"href": URLEditPage + "?route=" + route.URL.String}, hy.Txt("edit")),
				)
			case route.RedirectURL.Valid:
				div.Append("div", nil,
					hy.Txt("RedirectURL: "),
					hy.Txt(route.RedirectURL.String),
					hy.H("div", nil, hy.H("a", hy.Attr{"href": URLEditPage + "?route=" + route.URL.String}, hy.Txt("edit"))),
				)
			case route.HandlerURL.Valid:
				div.Append("div", nil,
					hy.Txt("HandlerURL: "),
					hy.Txt(route.HandlerURL.String),
					hy.H("div", nil, hy.H("a", hy.Attr{"href": URLEditPage + "?route=" + route.URL.String}, hy.Txt("edit"))),
				)
			case route.Content.Valid:
				div.Append("div", nil,
					hy.Txt("Content: &lt;some content&gt;"),
					hy.H("div", nil, hy.H("a", hy.Attr{"href": URLEditPage + "?route=" + route.URL.String}, hy.Txt("edit"))),
				)
			case route.ThemePath.Valid && route.Template.Valid:
				div.Append("div", nil,
					hy.Txt("ThemePath: "),
					hy.Txt(route.ThemePath.String),
					hy.Txt(", Template: "),
					hy.Txt(route.Template.String),
					hy.H("div", nil, hy.H("a", hy.Attr{"href": URLEditPage + "?route=" + route.URL.String}, hy.Txt("edit"))),
				)
			default:
				continue
			}
			els.AppendElements(div)
		}
		var tdata templateData
		tdata.Routes, err = hy.MarshalElement(nil, els)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		err = pm.executeTemplates(w, tdata, pagemanagerFS, "dashboard.html")
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
	case "POST":
		fallthrough
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
