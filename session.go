package pagemanager

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bokwoon95/pagemanager/erro"
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
	UserData     map[string]interface{}
}

func (user *User) RowMapper(USERS tables.PM_USERS) func(*sq.Row) error {
	return func(row *sq.Row) error {
		userID := row.NullInt64(USERS.USER_ID)
		user.Valid = userID.Valid
		user.UserID = userID.Int64
		user.PublicUserID = row.String(USERS.PUBLIC_USER_ID)
		user.LoginID = row.String(USERS.LOGIN_ID)
		user.Email = row.String(USERS.EMAIL)
		user.Displayname = row.String(USERS.DISPLAYNAME)
		b := row.Bytes(USERS.USER_DATA)
		return row.Accumulate(func() error {
			if len(b) > 0 {
				return erro.Wrap(json.Unmarshal(b, &user.UserData))
			}
			return nil
		})
	}
}

type SessionUser struct {
	User
	Roles       map[string]bool
	Permissions map[string]bool
	SessionData map[string]interface{}
}

func (user *SessionUser) RowMapper(u tables.PM_USERS, s tables.PM_SESSIONS) func(*sq.Row) error {
	return func(row *sq.Row) error {
		b := row.Bytes(s.SESSION_DATA)
		err := user.User.RowMapper(u)(row)
		if err != nil {
			return erro.Wrap(err)
		}
		return row.Accumulate(func() error {
			if len(b) > 0 {
				return erro.Wrap(json.Unmarshal(b, &user.SessionData))
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
		Name:  cookieSession,
		Value: b64SessionToken,
	})
	return nil
}

func (pm *PageManager) getSession(w http.ResponseWriter, r *http.Request) (user SessionUser, err error) {
	if !pm.boxesInitialized() {
		return user, ErrBoxesNotInitialized
	}
	c, _ := r.Cookie(cookieSession)
	if c == nil {
		return user, nil
	}
	sessionToken, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		http.SetCookie(w, &http.Cookie{Name: cookieSession, MaxAge: -1})
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
	var (
		SESSIONS         = tables.NEW_SESSIONS(r.Context(), "s")
		USERS            = tables.NEW_USERS(r.Context(), "u")
		USER_ROLES       = tables.NEW_USER_ROLES(r.Context(), "ur")
		USER_PERMISSIONS = tables.NEW_USER_PERMISSIONS(r.Context(), "up")
		ROLE_PERMISSIONS = tables.NEW_ROLE_PERMISSIONS(r.Context(), "rp")
	)
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
	user.Roles = make(map[string]bool)
	user.Permissions = make(map[string]bool)
	_, err = sq.Fetch(pm.dataDB, sq.SQLite.
		From(USERS).
		LeftJoin(USER_ROLES, USER_ROLES.USER_ID.Eq(USERS.USER_ID)).
		LeftJoin(USER_PERMISSIONS, USER_PERMISSIONS.USER_ID.Eq(USERS.USER_ID)).
		Join(ROLE_PERMISSIONS, ROLE_PERMISSIONS.ROLE_NAME.Eq(USER_ROLES.ROLE_NAME)).
		Where(USERS.USER_ID.EqInt64(user.UserID)).
		GroupBy(USERS.USER_ID),
		func(row *sq.Row) error {
			rolesBytes := row.Bytes(sq.Fieldf("json_group_array(?)", USER_ROLES.ROLE_NAME))
			userPermsBytes := row.Bytes(sq.Fieldf("json_group_array(?)", USER_PERMISSIONS.PERMISSION_NAME))
			rolePermsBytes := row.Bytes(sq.Fieldf("json_group_array(?)", ROLE_PERMISSIONS.PERMISSION_NAME))
			return row.Accumulate(func() error {
				var roles, userPerms, rolePerms []string
				err := json.Unmarshal(rolesBytes, &roles)
				if err != nil {
					return erro.Wrap(err)
				}
				err = json.Unmarshal(userPermsBytes, &userPerms)
				if err != nil {
					return erro.Wrap(err)
				}
				err = json.Unmarshal(rolePermsBytes, &rolePerms)
				if err != nil {
					return erro.Wrap(err)
				}
				for _, role := range roles {
					if role == "" {
						continue
					}
					user.Roles[role] = true
				}
				for _, perm := range userPerms {
					if perm == "" {
						continue
					}
					user.Permissions[perm] = true
				}
				for _, perm := range rolePerms {
					if perm == "" {
						continue
					}
					user.Permissions[perm] = true
				}
				return nil
			})
		},
	)
	return user, nil
}

func (pm *PageManager) getUser(w http.ResponseWriter, r *http.Request) (SessionUser, error) {
	user, ok := r.Context().Value(ctxKeyUser).(SessionUser)
	if ok {
		return user, nil
	}
	user, err := pm.getSession(w, r)
	if err != nil {
		user.Valid = false
	}
	return user, erro.Wrap(err)
}

func (pm *PageManager) deleteSession(w http.ResponseWriter, r *http.Request) error {
	defer http.SetCookie(w, &http.Cookie{Path: "/", Name: cookieSession, MaxAge: -1})
	if !pm.boxesInitialized() {
		return ErrBoxesNotInitialized
	}
	c, _ := r.Cookie(cookieSession)
	if c == nil {
		return nil
	}
	sessionToken, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		http.SetCookie(w, &http.Cookie{Name: cookieSession, MaxAge: -1})
		return erro.Wrap(err)
	}
	sessionHashes, err := pm.publicBox.HashAll(sessionToken)
	if err != nil {
		return erro.Wrap(err)
	}
	var b64SessionHashes []string
	for _, sessionHash := range sessionHashes {
		b64SessionHashes = append(b64SessionHashes, base64.RawURLEncoding.EncodeToString(sessionHash))
	}
	SESSIONS := tables.NEW_SESSIONS(r.Context(), "s")
	_, _, err = sq.Exec(pm.dataDB, sq.SQLite.
		DeleteFrom(SESSIONS).
		Where(SESSIONS.SESSION_HASH.In(b64SessionHashes)), 0,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	return nil
}
