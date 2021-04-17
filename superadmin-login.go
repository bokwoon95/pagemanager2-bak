package pagemanager

import (
	"net/http"
	"sync/atomic"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/encrypthash"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/hyforms"
	"github.com/bokwoon95/pagemanager/keyderiv"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
	"github.com/bokwoon95/pagemanager/templates"
)

type superadminLoginData struct {
	Password string
}

func (d *superadminLoginData) LoginForm(form *hyforms.Form) {
	password := form.
		Input("password", "pm-password", d.Password).
		Set("#pm-password.bg-near-white.pa2.w-100", hy.Attr{"required": hy.Enabled})

	form.Set(".bg-white.center-form", hy.Attr{"method": "POST"})
	if errMsgs := form.ErrMsgs(); len(errMsgs) > 0 {
		for _, errMsg := range errMsgs {
			form.Append("div.red", nil, hy.Txt(errMsg))
		}
	}
	form.Append("div.mt2.mb1.pt2", nil,
		hy.H("label.pointer", hy.Attr{"for": password.ID()}, hy.Txt("Superadmin Password:")))
	form.Append("div", nil, password)
	if hyforms.ErrMsgsMatch(password.ErrMsgs(), hyforms.RequiredErrMsg) {
		form.Append("div.f7.red", nil, hy.Txt(hyforms.RequiredErrMsg))
	}
	form.Append("div.mv2.pt2", nil, hy.H("button.pointer.pa2", hy.Attr{"type": "submit"}, hy.Txt("Log In")))

	form.Unmarshal(func() {
		d.Password = password.Validate(hyforms.Required).Value()
	})
}

func (pm *PageManager) superadminLogin(w http.ResponseWriter, r *http.Request) {
	var err error
	data := &superadminLoginData{}
	switch r.Method {
	case "GET":
		templateData := templates.CenterForm{
			Title:  "PageManager Login",
			Header: "PageManager Login",
		}
		templateData.Form, err = hyforms.MarshalForm(nil, w, r, data.LoginForm)
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
		errMsgs := hyforms.UnmarshalForm(w, r, data.LoginForm)
		if errMsgs.IsNonEmpty() {
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		var passwordHash []byte
		var keyParams []byte
		SUPERADMIN := tables.NEW_SUPERADMIN(r.Context(), "")
		_, err = sq.Fetch(pm.superadminDB, sq.SQLite.
			From(SUPERADMIN).
			Where(SUPERADMIN.ID.EqInt(1)),
			func(row *sq.Row) error {
				passwordHash = row.Bytes(SUPERADMIN.PASSWORD_HASH)
				keyParams = row.Bytes(SUPERADMIN.KEY_PARAMS)
				return nil
			},
		)
		if err != nil {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, erro.Wrap(err).Error())
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		err = keyderiv.CompareHashAndPassword(passwordHash, []byte(data.Password))
		if err != nil {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, "Invalid Password")
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		var params keyderiv.Params
		err = params.UnmarshalBinary(keyParams)
		if err != nil {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, erro.Wrap(err).Error())
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		key := params.DeriveKey([]byte(data.Password))
		pm.privateBox, err = encrypthash.NewStaticKey(key)
		if err != nil {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, erro.Wrap(err).Error())
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		pm.publicBox, err = encrypthash.NewRotatingKeys(pm.getKeys, pm.privateBox.Base64Decrypt)
		if err != nil {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, erro.Wrap(err).Error())
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		atomic.StoreInt32(&pm.privateBoxFlag, 1)
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
