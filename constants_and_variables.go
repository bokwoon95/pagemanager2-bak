package pagemanager

import (
	"database/sql"
	"errors"
)

var (
	ErrBoxesNotInitialized = errors.New("boxes not initialized")
)

type Route struct {
	URL         sql.NullString
	Disabled    sql.NullBool
	RedirectURL sql.NullString
	HandlerURL  sql.NullString
	Content     sql.NullString
	ThemePath   sql.NullString
	Template    sql.NullString
}

type ctxKey string

const (
	ctxKeyUser       ctxKey = "user"
	ctxKeyLocaleCode ctxKey = "localeCode"
)

const (
	roleSuperadmin = "pm-superadmin"
)

const (
	permissionPagePerms = "pm-page-perms"
)

const (
	URLDashboard       = "/pm-dashboard"
	URLSuperadminLogin = "/pm-superadmin-login"
	URLLogin           = "/pm-login"
)

const (
	cookieSuperadminSession       = "pm-superadmin-session"
	cookieSuperadminLoginRedirect = "pm-superadmin-login-redirect"
)

const (
	queryparamEditMode = "pm-edit"
)

const (
	EditModeOff      = ""
	EditModeBasic    = "basic"
	EditModeAdvanced = "advanced"
)
