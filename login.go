package pagemanager

import (
	"html/template"
	"net/http"

	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/hyforms"
	"github.com/bokwoon95/pagemanager/keyderiv"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
)

type loginData struct {
	LoginID  string
	Password string
}

func (d *loginData) LoginForm(form *hyforms.Form) {
	loginID := form.
		Text("pm-login-id", d.LoginID).
		Set("#pm-login-id.bg-near-white.pa2.w-100", hy.Attr{"required": hy.Enabled})
	password := form.
		Input("password", "pm-password", d.Password).
		Set("#pm-password.bg-near-white.pa2.w-100", hy.Attr{"required": hy.Enabled})

	form.Set(".bg-white.center-form", hy.Attr{"method": "POST"})
	if errMsgs := form.ErrMsgs(); len(errMsgs) > 0 {
		for _, errMsg := range errMsgs {
			form.Append("div.red", nil, hy.Txt(errMsg))
		}
	}
	form.Append("div.mt3.mb1", nil,
		hy.H("label.pointer", hy.Attr{"for": loginID.ID()}, hy.Txt("Email or Username:")))
	form.Append("div", nil, loginID)
	if hyforms.ErrMsgsMatch(loginID.ErrMsgs(), hyforms.RequiredErrMsg) {
		form.Append("div.f7.red", nil, hy.Txt(hyforms.RequiredErrMsg))
	}
	form.Append("div.mt3.mb1", nil,
		hy.H("label.pointer", hy.Attr{"for": password.ID()}, hy.Txt("Password:")))
	form.Append("div", nil, password)
	if hyforms.ErrMsgsMatch(password.ErrMsgs(), hyforms.RequiredErrMsg) {
		form.Append("div.f7.red", nil, hy.Txt(hyforms.RequiredErrMsg))
	}
	form.Append("div.mt3", nil, hy.H("button.pointer.pa2", hy.Attr{"type": "submit"}, hy.Txt("Log In")))

	form.Unmarshal(func() {
		d.LoginID = loginID.Validate(hyforms.Required).Value()
		d.Password = password.Validate(hyforms.Required).Value()
	})
}

func (pm *PageManager) login(w http.ResponseWriter, r *http.Request) {
	type templateData struct {
		Title  string
		Header template.HTML
		Form   template.HTML
	}
	data := &loginData{}
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
		err = pm.executeTemplates(w, tdata, pagemanagerFS, "login.html")
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
		var userID int64
		var passwordHash []byte
		USERS := tables.NEW_USERS(r.Context(), "u")
		rowCount, err := sq.Fetch(pm.superadminDB, sq.SQLite.
			From(USERS).
			Where(USERS.LOGIN_ID.EqString(data.LoginID)),
			func(row *sq.Row) error {
				userID = row.Int64(USERS.USER_ID)
				passwordHash = row.Bytes(USERS.PASSWORD_HASH)
				return nil
			},
		)
		if err != nil {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, err.Error())
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		if rowCount == 0 {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, "The login ID or password is incorrect")
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		err = keyderiv.CompareHashAndPassword(passwordHash, []byte(data.Password))
		if err != nil {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, "The login ID or password is incorrect")
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		err = pm.newSession(w, userID, nil)
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