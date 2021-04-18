package pagemanager

import (
	"net/http"
	"sync/atomic"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/hyforms"
)

type superadminDashboardData struct {
}

func (pm *PageManager) superadminDashboard(w http.ResponseWriter, r *http.Request) {
	var err error
	switch r.Method {
	case "GET":
		if atomic.LoadInt32(&pm.privateBoxFlag) != 1 {
			_ = hyforms.SetCookieValue(w, "pagemanager.superadmin-login-redirect", r.URL.Path, nil)
			http.Redirect(w, r, "/pm-superadmin/login", http.StatusMovedPermanently)
			return
		}
		err = executeTemplates(w, nil, pagemanagerFS, "superadmin-dashboard.html")
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
	case "POST":
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
