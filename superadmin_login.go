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

type superadminLoginData struct {
	w        http.ResponseWriter `json:"-"`
	r        *http.Request       `json:"-"`
	Title    string
	Header   template.HTML
	LoginID  string
	Password string
}

func (d *superadminLoginData) Form() (template.HTML, error) {
	return hyforms.MarshalForm(nil, d.w, d.r, d.loginForm)
}

func (d *superadminLoginData) loginForm(form *hyforms.Form) {
	loginID := form.
		Text("pm-login-id", d.LoginID).
		Set("#pm-login-id.bg-near-white.pa2.w-100", hy.Attr{})
	password := form.
		Input("password", "pm-password", d.Password).
		Set("#pm-password.bg-near-white.pa2.w-100", hy.Attr{"required": hy.Enabled})

	form.Set(".bg-washed-blue.center-form", hy.Attr{"method": "POST"})
	for _, errMsg := range form.ErrMsgs() {
		form.Append("div.red", nil, hy.Txt(errMsg))
	}
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer.i", hy.Attr{"for": loginID.ID()}, hy.Txt("Superadmin Login: "))),
		hy.H("div", nil, loginID),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer.i", hy.Attr{"for": password.ID()}, hy.Txt("Superadmin Password:"))),
		hy.H("div", nil, password),
	)
	if hyforms.ErrMsgsMatch(password.ErrMsgs(), hyforms.RequiredErrMsg) {
		form.Append("div.f7.red", nil, hy.Txt(hyforms.RequiredErrMsg))
	}
	form.Append("div.mt3", nil, hy.H("button.pointer.pa2", hy.Attr{"type": "submit"}, hy.Txt("Log In")))

	form.Unmarshal(func() {
		d.LoginID = loginID.Value()
		d.Password = password.Validate(hyforms.Required).Value()
	})
}

func (pm *PageManager) superadminLogin(w http.ResponseWriter, r *http.Request) {
	data := &superadminLoginData{w: w, r: r}
	var err error
	switch r.Method {
	case "GET":
		user := pm.getUser(w, r)
		if user.Valid && user.HasRole(roleSuperadmin) {
			Redirect(w, r, URLDashboard)
			return
		}
		data.Title = "PageManager Superadmin Login"
		data.Header = "PageManager Superadmin Login"
		err = pm.tpl.Render(w, r, data, tpl.Files("superadmin_login.html"))
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
	case "POST":
		errMsgs, ok := hyforms.UnmarshalForm(w, r, data.loginForm)
		if !ok {
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		SUPERADMIN := tables.NEW_SUPERADMIN("")
		exists, err := sq.Exists(pm.superadminDB, sq.SQLite.From(SUPERADMIN).Where(
			SUPERADMIN.ORDER_NUM.EqInt(1),
			SUPERADMIN.LOGIN_ID.EqString(data.LoginID),
		))
		if err != nil {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, err.Error())
			hyforms.Redirect(w, r, LocaleURL(r, r.URL.Path), errMsgs)
			return
		}
		if !exists {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, ErrInvalidLoginCredentials.Error())
			hyforms.Redirect(w, r, LocaleURL(r, r.URL.Path), errMsgs)
			return
		}
		err = pm.initializeBoxes([]byte(data.Password))
		if err != nil {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, err.Error())
			hyforms.Redirect(w, r, LocaleURL(r, r.URL.Path), errMsgs)
			return
		}
		err = pm.newSession(w, 1, nil)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		var redirectURL string
		_ = hyforms.GetCookieValue(w, r, cookieLoginRedirect, &redirectURL)
		if redirectURL != "" {
			Redirect(w, r, redirectURL)
			return
		}
		Redirect(w, r, URLDashboard)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
