package pagemanager

import (
	"html/template"
	"net/http"

	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/hyforms"
)

type superadminLoginData struct {
	LoginCode string
	Password  string
}

func (d *superadminLoginData) LoginForm(form *hyforms.Form) {
	logincode := form.
		Text("pm-login-code", d.Password).
		Set("#pm-login-code.bg-near-white.pa2.w-100", hy.Attr{})
	password := form.
		Input("password", "pm-password", d.Password).
		Set("#pm-password.bg-near-white.pa2.w-100", hy.Attr{"required": hy.Enabled})

	form.Set(".bg-washed-blue.center-form", hy.Attr{"method": "POST"})
	form.Append("div.mt3.mb1", nil,
		hy.H("label.pointer.i", hy.Attr{"for": logincode.ID()}, hy.Txt("Superadmin Login Code:")))
	form.Append("div", nil, logincode)
	if errMsgs := form.ErrMsgs(); len(errMsgs) > 0 {
		for _, errMsg := range errMsgs {
			form.Append("div.red", nil, hy.Txt(errMsg))
		}
	}
	form.Append("div.mt3.mb1", nil,
		hy.H("label.pointer.i", hy.Attr{"for": password.ID()}, hy.Txt("Superadmin Password:")))
	form.Append("div", nil, password)
	if hyforms.ErrMsgsMatch(password.ErrMsgs(), hyforms.RequiredErrMsg) {
		form.Append("div.f7.red", nil, hy.Txt(hyforms.RequiredErrMsg))
	}
	form.Append("div.mt3", nil, hy.H("button.pointer.pa2", hy.Attr{"type": "submit"}, hy.Txt("Log In")))

	form.Unmarshal(func() {
		d.LoginCode = logincode.Value()
		d.Password = password.Validate(hyforms.Required).Value()
	})
}

func (pm *PageManager) superadminLogin(w http.ResponseWriter, r *http.Request) {
	type templateData struct {
		Title  string
		Header template.HTML
		Form   template.HTML
	}
	data := &superadminLoginData{}
	var err error
	switch r.Method {
	case "GET":
		user := pm.getUser(w, r)
		if user.HasRole(roleSuperadmin) {
			Redirect(w, r, URLDashboard)
			return
		}
		tdata := templateData{
			Title:  "PageManager Login",
			Header: "PageManager Login",
		}
		tdata.Form, err = hyforms.MarshalForm(nil, w, r, data.LoginForm)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		err = pm.executeTemplates(w, tdata, pagemanagerFS, "superadmin_login.html")
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
	case "POST":
		errMsgs, ok := hyforms.UnmarshalForm(w, r, data.LoginForm)
		if !ok {
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		err = pm.initializeBoxes([]byte(data.Password))
		if err != nil {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, err.Error())
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		err = pm.newSession(w, 1, nil)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		var redirectURL string
		_ = hyforms.GetCookieValue(w, r, cookieSuperadminLoginRedirect, &redirectURL)
		if redirectURL != "" {
			Redirect(w, r, redirectURL)
			return
		}
		Redirect(w, r, URLDashboard)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
