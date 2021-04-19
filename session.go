package pagemanager

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
)

type User struct {
	Valid        bool
	UserID       int64
	PublicUserID string
	LoginID      string
	Email        string
	Displayname  string
	Roles        []string
	Permissions  map[string]interface{}
	UserData     map[string]interface{}
}

func (user *User) RowMapper(u tables.PM_USERS) func(*sq.Row) error {
	return func(row *sq.Row) error {
		userID := row.NullInt64(u.USER_ID)
		user.Valid = userID.Valid
		user.UserID = userID.Int64
		user.PublicUserID = row.String(u.PUBLIC_USER_ID)
		user.LoginID = row.String(u.LOGIN_ID)
		rolesBytes := row.Bytes(u.ROLES)
		permissionsBytes := row.Bytes(u.PERMISSIONS)
		return row.Accumulate(func() error {
			var err error
			if len(rolesBytes) > 0 {
				err = json.Unmarshal(rolesBytes, &user.Roles)
				if err != nil {
					return erro.Wrap(err)
				}
			}
			if len(permissionsBytes) > 0 {
				err = json.Unmarshal(permissionsBytes, &user.Permissions)
				if err != nil {
					return erro.Wrap(err)
				}
			}
			return nil
		})
	}
}

func (user *User) HasPagePerms(perms PagePerms) bool {
	p, _ := user.Permissions[permissionPagePerms].(float64)
	userPerms := PagePerms(int(p))
	return perms&userPerms != 0
}

func (user *User) HasRole(role string) bool {
	for _, r := range user.Roles {
		if role == r {
			return true
		}
	}
	return false
}

type SessionUser struct {
	User
	SessionData map[string]interface{}
}

func (user *SessionUser) RowMapper(u tables.PM_USERS, s tables.PM_SESSIONS) func(*sq.Row) error {
	return func(row *sq.Row) error {
		rawSessionData := row.Bytes(s.SESSION_DATA)
		err := user.User.RowMapper(u)(row)
		if err != nil {
			return erro.Wrap(err)
		}
		return row.Accumulate(func() error {
			if len(rawSessionData) > 0 {
				err = json.Unmarshal(rawSessionData, &user.SessionData)
				if err != nil {
					return erro.Wrap(err)
				}
			}
			return nil
		})
	}
}

func (pm *PageManager) newSession(w http.ResponseWriter, userID int64, sessionData map[string]interface{}) error {
	if !pm.boxesInitialized() {
		return ErrBoxesNotInitialized
	}
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
			col.SetTime(SESSIONS.CREATED_AT, time.Now())
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

func (pm *PageManager) getSession(w http.ResponseWriter, r *http.Request) (user SessionUser, err error) {
	if !pm.boxesInitialized() {
		return user, ErrBoxesNotInitialized
	}
	c, _ := r.Cookie("pm-session")
	if c == nil {
		return user, nil
	}
	sessionToken, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		http.SetCookie(w, &http.Cookie{Name: "pm-session", MaxAge: -1})
		return user, erro.Wrap(err)
	}
	sessionHashes, err := pm.publicBox.HashAll(sessionToken)
	if err != nil {
		return user, erro.Wrap(err)
	}
	var b64SessionHashes []string
	for _, sessionHash := range sessionHashes {
		b64SessionHashes = append(b64SessionHashes, base64.RawURLEncoding.EncodeToString(sessionHash))
	}
	SESSIONS, USERS := tables.NEW_SESSIONS(r.Context(), "s"), tables.NEW_USERS(r.Context(), "u")
	_, err = sq.Fetch(pm.dataDB, sq.SQLite.
		From(SESSIONS).
		Join(USERS, USERS.USER_ID.Eq(SESSIONS.USER_ID)).
		Where(SESSIONS.SESSION_HASH.In(b64SessionHashes)).
		Limit(1),
		user.RowMapper(USERS, SESSIONS),
	)
	if err != nil {
		return user, erro.Wrap(err)
	}
	data := make(map[string]interface{})
	if len(user.Roles) > 0 {
		AUTHZ_GROUPS := tables.NEW_ROLES(r.Context(), "ag")
		ord := sq.Case(AUTHZ_GROUPS.NAME)
		for i, group := range user.Roles {
			ord = ord.When(group, i+1)
		}
		_, err = sq.Fetch(pm.dataDB, sq.SQLite.
			From(AUTHZ_GROUPS).
			Where(AUTHZ_GROUPS.NAME.In(user.Roles)).
			OrderBy(ord),
			func(row *sq.Row) error {
				b := row.Bytes(AUTHZ_GROUPS.PERMISSIONS)
				return row.Accumulate(func() error {
					var m map[string]interface{}
					err = json.Unmarshal(b, &m)
					if err != nil {
						return erro.Wrap(err)
					}
					for k, v := range m {
						data[k] = v
					}
					return nil
				})
			},
		)
		if err != nil {
			return user, erro.Wrap(err)
		}
	}
	for k, v := range user.Permissions {
		data[k] = v
	}
	user.Permissions = data
	return user, nil
}

func (pm *PageManager) getUser(w http.ResponseWriter, r *http.Request) SessionUser {
	user, ok := r.Context().Value(ctxKeyUser).(SessionUser)
	if ok {
		return user
	}
	user, err := pm.getSession(w, r)
	if err != nil {
		user.Valid = false
	}
	return user
}
