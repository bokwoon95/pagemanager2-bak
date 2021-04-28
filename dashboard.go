package pagemanager

import (
	"html/template"
	"net/http"

	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/hyforms"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
	"github.com/bokwoon95/pagemanager/tpl"
)

type dashboardData struct {
	w     http.ResponseWriter `json:"-"`
	r     *http.Request       `json:"-"`
	Pages []Page
}

func (d *dashboardData) PagesList() (template.HTML, error) {
	var els hy.Elements
	els.Append("div.mv2", nil, hy.H("a", hy.Attr{"href": URLCreatePage}, hy.Txt("create")))
	for _, page := range d.Pages {
		div := hy.H("div.mv2", nil)
		div.Append("div", nil, hy.Txt("URL: ", page.URL))
		if page.URL == "" {
			continue
		}
		switch page.PageType {
		case PageTypeDisabled:
			div.Append("div", nil,
				hy.Txt("Disabled:", page.Hidden),
				hy.H("a", hy.Attr{"href": URLEditPage + "?url=" + page.URL}, hy.Txt("edit")),
			)
		case PageTypeRedirect:
			div.Append("div", nil,
				hy.Txt("RedirectURL:", page.RedirectURL),
				hy.H("div", nil, hy.H("a", hy.Attr{"href": URLEditPage + "?url=" + page.URL}, hy.Txt("edit"))),
			)
		case PageTypePlugin:
			div.Append("div", nil,
				hy.Txt("HandlerURL:", page.HandlerName),
				hy.H("div", nil, hy.H("a", hy.Attr{"href": URLEditPage + "?url=" + page.URL}, hy.Txt("edit"))),
			)
		case PageTypeContent:
			div.Append("div", nil,
				hy.Txt("Content: <some content>"),
				hy.H("div", nil, hy.H("a", hy.Attr{"href": URLEditPage + "?url=" + page.URL}, hy.Txt("edit"))),
			)
		case PageTypeTemplate:
			div.Append("div", nil,
				hy.Txt("ThemePath:", page.ThemePath+", Template:", page.TemplateName),
				hy.H("div", nil, hy.H("a", hy.Attr{"href": URLEditPage + "?url=" + page.URL}, hy.Txt("edit"))),
			)
		default:
			continue
		}
		els.AppendElements(div)
	}
	return hy.Marshal(els)
}

func (pm *PageManager) dashboard(w http.ResponseWriter, r *http.Request) {
	data := &dashboardData{w: w, r: r}
	r.ParseForm()
	switch r.Method {
	case "GET":
		user := pm.getUser(w, r)
		switch {
		case !user.Valid:
			_ = hyforms.SetCookieValue(w, cookieLoginRedirect, LocaleURL(r, r.URL.Path), nil)
			pm.RedirectToLogin(w, r)
			return
		case !user.HasPermission(permissionViewPage):
			pm.Forbidden(w, r)
			return
		}
		PAGES := tables.NEW_PAGES(r.Context(), "r")
		_, err := sq.Fetch(pm.dataDB, sq.SQLite.From(PAGES).OrderBy(PAGES.URL), func(row *sq.Row) error {
			var page Page
			if err := page.RowMapper(PAGES)(row); err != nil {
				return erro.Wrap(err)
			}
			return row.Accumulate(func() error {
				data.Pages = append(data.Pages, page)
				return nil
			})
		})
		err = pm.tpl.Render(w, r, data, tpl.Files("dashboard.html"))
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
