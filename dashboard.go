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
	Pages []Page
}

func (pm *PageManager) dashboard(w http.ResponseWriter, r *http.Request) {
	type templateData struct {
		Pages template.HTML
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
			var page Page
			if err := page.RowMapper(p)(row); err != nil {
				return erro.Wrap(err)
			}
			return row.Accumulate(func() error {
				data.Pages = append(data.Pages, page)
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
		for _, page := range data.Pages {
			div := hy.H("div.mv2", nil)
			div.Append("div", nil, hy.Txt("URL: "), hy.Txt(page.URL))
			if page.URL == "" {
				continue
			}
			switch page.PageType {
			case PageTypeDisabled:
				div.Append("div", nil,
					hy.Txt("Disabled: "),
					hy.Txt(page.Disabled),
					hy.H("a", hy.Attr{"href": URLEditPage + "?url=" + page.URL}, hy.Txt("edit")),
				)
			case PageTypeRedirect:
				div.Append("div", nil,
					hy.Txt("RedirectURL: "),
					hy.Txt(page.RedirectURL),
					hy.H("div", nil, hy.H("a", hy.Attr{"href": URLEditPage + "?url=" + page.URL}, hy.Txt("edit"))),
				)
			case PageTypePlugin:
				div.Append("div", nil,
					hy.Txt("HandlerURL: "),
					hy.Txt(page.HandlerURL),
					hy.H("div", nil, hy.H("a", hy.Attr{"href": URLEditPage + "?url=" + page.URL}, hy.Txt("edit"))),
				)
			case PageTypeContent:
				div.Append("div", nil,
					hy.Txt("Content: &lt;some content&gt;"),
					hy.H("div", nil, hy.H("a", hy.Attr{"href": URLEditPage + "?url=" + page.URL}, hy.Txt("edit"))),
				)
			case PageTypeTemplate:
				div.Append("div", nil,
					hy.Txt("ThemePath: "),
					hy.Txt(page.ThemePath),
					hy.Txt(", Template: "),
					hy.Txt(page.Template),
					hy.H("div", nil, hy.H("a", hy.Attr{"href": URLEditPage + "?url=" + page.URL}, hy.Txt("edit"))),
				)
			default:
				continue
			}
			els.AppendElements(div)
		}
		var tdata templateData
		tdata.Pages, err = hy.MarshalElement(nil, els)
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
