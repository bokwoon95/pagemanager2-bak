package pagemanager

import (
	"crypto/rand"
	"html/template"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/bokwoon95/pagemanager/encrypthash"
	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/hyforms"
	"github.com/bokwoon95/pagemanager/keyderiv"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
)

type superadminSetupData struct {
	LoginCode       string
	Password        string
	ConfirmPassword string
}

func (d *superadminSetupData) setupForm(form *hyforms.Form) {
	const passwordNotMatch = "passwords do not match"
	logincode := form.
		Text("pm-login-code", d.LoginCode).
		Set("#pm-password.bg-near-white.pa2.w-100", hy.Attr{"title": "Login Code"})
	password := form.
		Input("password", "pm-password", d.Password).
		Set("#pm-password.bg-near-white.pa2.w-100", hy.Attr{
			"required": hy.Enabled,
			"title":    "Superadmin Password",
		})
	confirmPassword := form.
		Input("password", "pm-confirm-password", d.ConfirmPassword).
		Set("#pm-confirm-password.bg-near-white.pa2.w-100", hy.Attr{
			"required": hy.Enabled,
			"title":    "Confirm Superadmin Password",
		})

	form.Set(".bg-white.setup-form", hy.Attr{"method": "POST"})
	form.AppendElements(hy.Elements{
		hy.H("div.f4", nil, hy.Txt("No Superadmin detected.")),
		hy.H("div.f6", nil, hy.Txt("To make changes to your website, you need to create a Superadmin account.")),
	})
	form.AppendElements(hy.Elements{
		hy.H("div.mt4.i", nil, hy.Txt("Login Code")),
		hy.H("hr.mv1", nil),
		hy.H("div.f6", nil, hy.Txt("If you are deploying PageManager on the internet, it is recommended that you use one of the suggested login codes. Otherwise if you are only using PageManager offline, you can leave the login code blank.")),
	})
	form.Append("div.mt3.mb1", nil,
		hy.H("label.pointer", hy.Attr{"for": logincode.ID()}, hy.Txt("Superadmin Login Code: (optional)")))
	form.Append("div", nil, logincode)
	form.Append("div", nil, hy.Txt("Suggestion: apple demure ace"))

	form.AppendElements(hy.Elements{
		hy.H("div.mt4.i", nil, hy.Txt("Password")),
		hy.H("hr.mv1", nil),
	})
	form.Append("div.mt3.mb1", nil,
		hy.H("label.pointer", hy.Attr{"for": password.ID()}, hy.Txt("Superadmin Password:")))
	form.Append("div", nil, password)
	if hyforms.ErrMsgsMatch(password.ErrMsgs(), hyforms.RequiredErrMsg) {
		form.Append("div.f7.red", nil, hy.Txt(hyforms.RequiredErrMsg))
	}
	form.Append("div.mt3.mb1", nil,
		hy.H("label.pointer", hy.Attr{"for": confirmPassword.ID()}, hy.Txt("Confirm Superadmin Password:")))
	form.Append("div", nil, confirmPassword)
	if hyforms.ErrMsgsMatch(confirmPassword.ErrMsgs(), passwordNotMatch) {
		form.Append("div.f7.red", nil, hy.Txt(passwordNotMatch))
	}
	form.Append("div.mt3", nil, hy.H("button.pointer.pa2", hy.Attr{"type": "submit"}, hy.Txt("Create Superadmin")))

	form.Unmarshal(func() {
		d.LoginCode = logincode.Validate().Value()
		d.Password = password.Validate(hyforms.Required).Value()
		d.ConfirmPassword = confirmPassword.Value()
		if d.ConfirmPassword != d.Password {
			form.AddInputErrMsgs(confirmPassword.Name(), passwordNotMatch)
		}
	})
}

func (pm *PageManager) superadminSetup(w http.ResponseWriter, r *http.Request) {
	type templateData struct {
		Title  string
		Header template.HTML
		Form   template.HTML
	}
	data := &superadminSetupData{}
	const setupForm = "setupForm"
	var err error
	switch r.Method {
	case "GET":
		tdata := templateData{
			Title:  "PageManager Setup",
			Header: "PageManager Setup",
		}
		_ = hyforms.GetCookieValue(w, r, setupForm, data)
		tdata.Form, err = hyforms.MarshalForm(nil, w, r, data.setupForm)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		err = pm.executeTemplates(w, tdata, pagemanagerFS, "superadmin_setup.html")
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
	case "POST":
		errMsgs, ok := hyforms.UnmarshalForm(w, r, data.setupForm)
		if !ok {
			_ = hyforms.SetCookieValue(w, setupForm, data, nil)
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		passwordHash, err := keyderiv.GenerateFromPassword([]byte(data.Password))
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		params, err := keyderiv.NewParams()
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		key := params.DeriveKey([]byte(data.Password))
		pm.privateBox, err = encrypthash.NewStaticKey(key)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		atomic.StoreInt32(&pm.privateBoxFlag, 1)
		SUPERADMIN := tables.NEW_SUPERADMIN(r.Context(), "")
		_, _, err = sq.Exec(pm.superadminDB, sq.SQLite.
			InsertInto(SUPERADMIN).
			Valuesx(func(col *sq.Column) error {
				col.SetInt(SUPERADMIN.ORDER_NUM, 1)
				col.SetString(SUPERADMIN.PASSWORD_HASH, string(passwordHash))
				col.SetString(SUPERADMIN.KEY_PARAMS, params.String())
				return nil
			}), 0,
		)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		key = make([]byte, 32)
		_, err = rand.Read(key)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		keyCiphertext, err := pm.privateBox.Base64Encrypt(key)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		KEYS := tables.NEW_KEYS(r.Context(), "")
		_, _, err = sq.Exec(pm.superadminDB, sq.SQLite.DeleteFrom(KEYS), 0)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		_, _, err = sq.Exec(pm.superadminDB, sq.SQLite.
			InsertInto(KEYS).
			Valuesx(func(col *sq.Column) error {
				col.SetInt(KEYS.ORDER_NUM, 1)
				col.SetString(KEYS.KEY_CIPHERTEXT, string(keyCiphertext))
				col.SetTime(KEYS.CREATED_AT, time.Now())
				return nil
			}), 0,
		)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		Redirect(w, r, URLSuperadminLogin)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
