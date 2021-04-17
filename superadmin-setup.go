package pagemanager

import (
	"net/http"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/hyforms"
	"github.com/bokwoon95/pagemanager/templates"
)

type superadminSetupData struct {
	Password        string
	ConfirmPassword string
}

func (d *superadminSetupData) setupForm(form *hyforms.Form) {
	const passwordNotMatch = "password does not match"
	password := form.
		Input("password", "pm-password", d.Password).
		Set("", hy.Attr{"required": hy.Enabled})
	confirmPassword := form.
		Input("password", "pm-confirm-password", d.ConfirmPassword).
		Set("", hy.Attr{"required": hy.Enabled})

	form.Set(".bg-white", hy.Attr{"method": "POST"})
	form.Append("div", nil, hy.Txt("To make changes to your website, you need to create a Superadmin account"))
	form.Append("div.mv2.pt2", nil, hy.H("label.pointer", hy.Attr{"for": password.ID()}, hy.Txt("Password:")))
	form.Append("div", nil, password)
	if hyforms.ErrMsgsMatch(password.ErrMsgs(), hyforms.RequiredErrMsg) {
		form.Append("div.f7.red", nil, hy.Txt(hyforms.RequiredErrMsg))
	}
	form.Append("div.mv2.pt2", nil, hy.H("label.pointer", hy.Attr{"for": confirmPassword.ID()}, hy.Txt("Confirm Password:")))
	form.Append("div", nil, confirmPassword)
	if hyforms.ErrMsgsMatch(confirmPassword.ErrMsgs(), passwordNotMatch) {
		form.Append("div.f7.red", nil, hy.Txt(passwordNotMatch))
	}
	form.Append("div.mv2.pt2", nil, hy.H("button.pointer", hy.Attr{"type": "submit"}, hy.Txt("Create Superadmin")))

	form.Unmarshal(func() {
		d.Password = password.Validate(hyforms.Required).Value()
		d.ConfirmPassword = confirmPassword.Value()
		if d.ConfirmPassword != d.Password {
			form.AddInputErrMsgs(confirmPassword.Name(), passwordNotMatch)
		}
	})
}

func (pm *PageManager) superadminSetup(w http.ResponseWriter, r *http.Request) {
	const setupForm = "setupForm"
	var err error
	data := &superadminSetupData{}
	switch r.Method {
	case "GET":
		templateData := templates.CenterForm{
			Title:  "Superadmin Setup",
			Header: "Superadmin Setup",
		}
		_ = hyforms.CookiePop(w, r, setupForm, data)
		templateData.Form, err = hyforms.MarshalForm(nil, w, r, data.setupForm)
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
		err = executeTemplates(w, templateData, pagemanagerFS, "templates/center-form.html")
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
	case "POST":
		err := hyforms.UnmarshalForm(w, r, data.setupForm)
		if err != nil {
			_ = hyforms.CookieSet(w, setupForm, *data, nil)
			http.Redirect(w, r, r.URL.Path, http.StatusMovedPermanently)
			return
		}
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
