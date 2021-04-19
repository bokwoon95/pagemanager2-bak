package pagemanager

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/hyforms"
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
	form.Append("div.mt3.mb1", nil,
		hy.H("label.pointer", hy.Attr{"for": password.ID()}, hy.Txt("Superadmin Password:")))
	form.Append("div", nil, password)
	if hyforms.ErrMsgsMatch(password.ErrMsgs(), hyforms.RequiredErrMsg) {
		form.Append("div.f7.red", nil, hy.Txt(hyforms.RequiredErrMsg))
	}
	form.Append("div.mt3", nil, hy.H("button.pointer.pa2", hy.Attr{"type": "submit"}, hy.Txt("Log In")))

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
		errMsgs, ok := hyforms.UnmarshalForm(w, r, data.LoginForm)
		if !ok {
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		err = pm.setSuperadminPassword([]byte(data.Password))
		if err != nil {
			errMsgs.FormErrMsgs = append(errMsgs.FormErrMsgs, err.Error())
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		err = pm.newSession(w, 1, nil)
		if err != nil {
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
			return
		}
		var redirectURL string
		_ = hyforms.GetCookieValue(w, r, "pagemanager.superadmin-login-redirect", &redirectURL)
		if redirectURL != "" {
			http.Redirect(w, r, redirectURL, http.StatusMovedPermanently)
			return
		}
		http.Redirect(w, r, "/pm-superadmin", http.StatusMovedPermanently)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func (pm *PageManager) newSession(w http.ResponseWriter, userID int64, sessionData interface{}) error {
	sessionToken := make([]byte, 24)
	_, err := rand.Read(sessionToken)
	if err != nil {
		return erro.Wrap(err)
	}
	sessionHash, err := pm.publicBox.Hash(sessionToken)
	if err != nil {
		return erro.Wrap(err)
	}
	b64SessionToken := base64.RawURLEncoding.EncodeToString(sessionToken)
	b64SessionHash := base64.RawURLEncoding.EncodeToString(sessionHash)
	var b []byte
	if sessionData != nil {
		b, err = json.Marshal(sessionData)
		if err != nil {
			return erro.Wrap(err)
		}
	}
	ctx := context.Background()
	SESSIONS := tables.NEW_SESSIONS(ctx, "s")
	_, _, err = sq.Exec(pm.dataDB, sq.SQLite.DeleteFrom(SESSIONS).Where(SESSIONS.USER_ID.EqInt64(userID)), 0) // optional
	if err != nil {
		return erro.Wrap(err)
	}
	_, _, err = sq.Exec(pm.dataDB, sq.SQLite.
		InsertInto(SESSIONS).
		Valuesx(func(col *sq.Column) error {
			col.SetInt64(SESSIONS.USER_ID, userID)
			col.SetString(SESSIONS.SESSION_HASH, b64SessionHash)
			if len(b) > 0 {
				col.Set(SESSIONS.SESSION_DATA, string(b))
			}
			return nil
		}), 0,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	http.SetCookie(w, &http.Cookie{
		Path:  "/",
		Name:  "pm-session",
		Value: b64SessionToken,
	})
	return nil
}

func (pm *PageManager) getSession(w http.ResponseWriter, r *http.Request) (userID int64, authzData, sessionData map[string]interface{}, err error) {
	c, _ := r.Cookie("pm-session")
	if c == nil {
		return 0, nil, nil, nil
	}
	sessionToken, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		http.SetCookie(w, &http.Cookie{Name: "pm-session", MaxAge: -1})
		return 0, nil, nil, erro.Wrap(err)
	}
	sessionHashes, err := pm.publicBox.HashAll(sessionToken)
	if err != nil {
		return 0, nil, nil, erro.Wrap(err)
	}
	var b64SessionHashes []string
	for _, sessionHash := range sessionHashes {
		b64SessionHashes = append(b64SessionHashes, base64.RawURLEncoding.EncodeToString(sessionHash))
	}
	var authzGroups []interface{}
	SESSIONS, USERS := tables.NEW_SESSIONS(r.Context(), "s"), tables.NEW_USERS(r.Context(), "u")
	_, err = sq.Fetch(pm.dataDBL(), sq.SQLite.
		From(SESSIONS).
		Join(USERS, USERS.USER_ID.Eq(SESSIONS.USER_ID)).
		Where(SESSIONS.SESSION_HASH.In(b64SessionHashes)).
		Limit(1),
		func(row *sq.Row) error {
			userID = row.Int64(SESSIONS.USER_ID)
			b1 := row.Bytes(SESSIONS.SESSION_DATA)
			b2 := row.Bytes(USERS.AUTHZ_GROUPS)
			b3 := row.Bytes(USERS.AUTHZ_DATA)
			return row.Accumulate(func() error {
				if len(b1) > 0 {
					_ = json.Unmarshal(b1, &sessionData)
				}
				if len(b2) > 0 {
					_ = json.Unmarshal(b2, &authzGroups)
				}
				if len(b3) > 0 {
					_ = json.Unmarshal(b3, &authzData)
				}
				return nil
			})
		},
	)
	if err != nil {
		return 0, nil, nil, erro.Wrap(err)
	}
	AUTHZ_GROUPS := tables.NEW_AUTHZ_GROUPS(r.Context(), "ag")
	ord := sq.Case(AUTHZ_GROUPS.NAME)
	var i int
	for _, group := range authzGroups {
		i++
		ord = ord.When(group, i)
	}
	data := make(map[string]interface{})
	_, err = sq.Fetch(pm.dataDBL(), sq.SQLite.
		From(AUTHZ_GROUPS).
		Where(AUTHZ_GROUPS.NAME.In(authzGroups)).
		OrderBy(ord),
		func(row *sq.Row) error {
			b := row.Bytes(AUTHZ_GROUPS.AUTHZ_DATA)
			return row.Accumulate(func() error {
				var m map[string]interface{}
				_ = json.Unmarshal(b, &m)
				for k, v := range m {
					data[k] = v
				}
				return nil
			})
		},
	)
	if err != nil {
		return 0, nil, nil, erro.Wrap(err)
	}
	for k, v := range authzData {
		data[k] = v
	}
	authzData = data
	return userID, authzData, sessionData, nil
}
