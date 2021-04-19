package pagemanager

import (
	"net/http"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/hyforms"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
)

type dashboardData struct {
	Routes []Route
}

func (pm *PageManager) dashboard(w http.ResponseWriter, r *http.Request) {
	var err error
	switch r.Method {
	case "GET":
		user := pm.getUser(w, r)
		if !user.HasPagePerms(PagePermsRead) {
			_ = hyforms.SetCookieValue(w, cookieSuperadminLoginRedirect, r.URL.Path, nil)
			http.Redirect(w, r, LocaleURL(r, URLSuperadminLogin), http.StatusMovedPermanently)
			return
		}
		data := dashboardData{}
		p := tables.NEW_PAGES(r.Context(), "r")
		_, err = sq.Fetch(pm.dataDB, sq.SQLite.From(p), func(row *sq.Row) error {
			var route Route
			if err := route.RowMapper(p)(row); err != nil {
				return erro.Wrap(err)
			}
			return row.Accumulate(func() error {
				data.Routes = append(data.Routes, route)
				return nil
			})
		})
		err = pm.executeTemplatesV2(w, r, data, pagemanagerFS, "dashboard.html")
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
	case "POST":
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
