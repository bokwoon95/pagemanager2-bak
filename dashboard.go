package pagemanager

import (
	"net/http"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/hyforms"
)

type dashboardData struct {
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
		err = pm.executeTemplates(w, nil, pagemanagerFS, "dashboard.html")
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
	case "POST":
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
