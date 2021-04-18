package pagemanager

import (
	"crypto/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/encrypthash"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/hyforms"
	"github.com/bokwoon95/pagemanager/keyderiv"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
	"github.com/bokwoon95/pagemanager/templates"
)

type superadminSetupData struct {
	Password        string
	ConfirmPassword string
}

func (d *superadminSetupData) setupForm(form *hyforms.Form) {
	const passwordNotMatch = "passwords do not match"
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

	form.Set(".bg-white.center-form", hy.Attr{"method": "POST"})
	form.Append("div.f4", nil,
		hy.Txt("No Superadmin detected."))
	form.Append("div.f6", nil,
		hy.Txt("To make changes to your website, you need to create a Superadmin account."))
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
			Title:  "PageManager Setup",
			Header: "PageManager Setup",
		}
		_ = hyforms.GetCookieValue(w, r, setupForm, data)
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
		errMsgs := hyforms.UnmarshalForm(w, r, data.setupForm)
		if errMsgs.IsNonEmpty() {
			_ = hyforms.SetCookieValue(w, setupForm, data, nil)
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		passwordHash, err := keyderiv.GenerateFromPassword([]byte(data.Password))
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
		params, err := keyderiv.NewParams()
		key := params.DeriveKey([]byte(data.Password))
		pm.privateBox, err = encrypthash.NewStaticKey(key)
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
		atomic.StoreInt32(&pm.privateBoxFlag, 1)
		SUPERADMIN := tables.NEW_SUPERADMIN(r.Context(), "")
		_, _, err = sq.Exec(pm.superadminDB, sq.SQLite.
			InsertInto(SUPERADMIN).
			Valuesx(func(col *sq.Column) error {
				col.SetInt(SUPERADMIN.ID, 1)
				col.SetString(SUPERADMIN.PASSWORD_HASH, string(passwordHash))
				col.SetString(SUPERADMIN.KEY_PARAMS, params.String())
				return nil
			}), 0,
		)
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
		key = make([]byte, 32)
		_, err = rand.Read(key)
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
		keyCiphertext, err := pm.privateBox.Base64Encrypt(key)
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
		KEYS := tables.NEW_KEYS(r.Context(), "")
		_, _, err = sq.Exec(pm.superadminDB, sq.SQLite.DeleteFrom(KEYS), 0)
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
		_, _, err = sq.Exec(pm.superadminDB, sq.SQLite.
			InsertInto(KEYS).
			Valuesx(func(col *sq.Column) error {
				col.SetInt(KEYS.ID, 1)
				col.SetString(KEYS.KEY_CIPHERTEXT, string(keyCiphertext))
				col.SetTime(KEYS.CREATED_AT, time.Now())
				return nil
			}), 0,
		)
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
