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
	"github.com/bokwoon95/pagemanager/tpl"
)

type superadminSetupData struct {
	w               http.ResponseWriter `json:"-"`
	r               *http.Request       `json:"-"`
	Title           string
	Header          template.HTML
	LoginID         string
	Password        string
	ConfirmPassword string
}

func (d *superadminSetupData) Form() (template.HTML, error) {
	return hyforms.MarshalForm(d.w, d.r, d.setupForm)
}

func (d *superadminSetupData) setupForm(form *hyforms.Form) {
	const passwordNotMatch = "passwords do not match"
	loginID := form.
		Text("pm-login-id", d.LoginID).
		Set("#pm-login-id.bg-near-white.pa2.w-100", hy.Attr{})
	password := form.
		Input("password", "pm-password", d.Password).
		Set("#pm-password.bg-near-white.pa2.w-100", hy.Attr{"required": hy.Enabled})
	confirmPassword := form.
		Input("password", "pm-confirm-password", d.ConfirmPassword).
		Set("#pm-confirm-password.bg-near-white.pa2.w-100", hy.Attr{"required": hy.Enabled})

	form.Set(".bg-white.setup-form", hy.Attr{"method": "POST"})
	form.AppendElements(hy.Elements{
		hy.H("div.f4", nil, hy.Txt("No Superadmin detected.")),
		hy.H("div.f6", nil, hy.Txt("To make changes to your website, you need to create a Superadmin account.")),
	})
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": loginID.ID()},
			hy.Txt("Email or Username: "), hy.H("span.f6.gray", nil, hy.Txt("(optional)")),
		)),
		hy.H("div", nil, loginID),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": password.ID()}, hy.Txt("Superadmin Password:"))),
		hy.H("div", nil, password),
	)
	if hyforms.ErrMsgsMatch(password.ErrMsgs(), hyforms.RequiredErrMsg) {
		form.Append("div.f7.red", nil, hy.Txt(hyforms.RequiredErrMsg))
	}
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": confirmPassword.ID()}, hy.Txt("Confirm Superadmin Password:"))),
		hy.H("div", nil, confirmPassword),
	)
	if hyforms.ErrMsgsMatch(confirmPassword.ErrMsgs(), passwordNotMatch) {
		form.Append("div.f7.red", nil, hy.Txt(passwordNotMatch))
	}
	form.Append("div.mt3", nil, hy.H("button.pointer.pa2", hy.Attr{"type": "submit"}, hy.Txt("Create Superadmin")))

	form.Unmarshal(func() {
		d.LoginID = loginID.Value()
		d.Password = password.Validate(hyforms.Required).Value()
		d.ConfirmPassword = confirmPassword.Value()
		if d.ConfirmPassword != d.Password {
			form.AddInputErrMsgs(confirmPassword.Name(), passwordNotMatch)
		}
	})
}

func (pm *PageManager) superadminSetup(w http.ResponseWriter, r *http.Request) {
	data := &superadminSetupData{w: w, r: r}
	const setupForm = "setupForm"
	var err error
	switch r.Method {
	case "GET":
		data.Title = "PageManager Setup"
		data.Header = "PageManager Setup"
		_ = hyforms.GetCookieValue(w, r, setupForm, data)
		err = pm.tpl.Render(w, r, data, tpl.Files("superadmin_setup.html"))
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
		SUPERADMIN := tables.NEW_SUPERADMIN("")
		_, _, err = sq.Exec(pm.superadminDB, sq.SQLite.
			InsertInto(SUPERADMIN).
			Valuesx(func(col *sq.Column) error {
				col.SetInt(SUPERADMIN.ORDER_NUM, 1)
				col.SetString(SUPERADMIN.LOGIN_ID, data.LoginID)
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
		KEYS := tables.NEW_KEYS("")
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
