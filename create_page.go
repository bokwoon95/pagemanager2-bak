package pagemanager

import (
	"fmt"
	"net/http"

	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/hyforms"
)

type createPageData struct {
	URL      string
	Disabled bool
}

func (pm *PageManager) createPage(w http.ResponseWriter, r *http.Request) {
	type templateData struct {
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
		case !user.HasPagePerms(PagePermsCreate):
			pm.Forbidden(w, r)
			return
		}
		err = pm.executeTemplates(w, nil, pagemanagerFS, "create_page.html")
		if err != nil {
			fmt.Println(err)
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
	case "POST":
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
