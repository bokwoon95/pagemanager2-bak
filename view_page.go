package pagemanager

import (
	"html/template"
	"net/http"

	"github.com/bokwoon95/pagemanager/hy"
)

type viewPageData struct {
	w http.ResponseWriter `json:"-"`
	r *http.Request       `json:"-"`
	Page
}

func (data *viewPageData) JS() (template.HTML, error) {
	return hy.Marshal(InlinedJS(data.w, pagemanagerFS, []string{"view_page.js"}))
}

func (pm *PageManager) viewPage(w http.ResponseWriter, r *http.Request) {
	data := &viewPageData{w: w, r: r}
	_ = data
	r.ParseForm()
	switch r.Method {
	case "GET":
		user, _ := pm.getUser(w, r)
		switch {
		case !user.Valid:
			pm.RedirectToLogin(w, r)
			return
		case !user.Permissions[permissionViewPage]:
			pm.Forbidden(w, r)
			return
		}
	case "POST":
		Redirect(w, r, r.URL.Path)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
