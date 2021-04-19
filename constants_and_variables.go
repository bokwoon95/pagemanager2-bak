package pagemanager

import (
	"database/sql"
	"errors"

	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
)

// There's always a lighthouse. There's always a man. There's always a city.

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

func (r *Route) RowMapper(p tables.PM_PAGES) func(*sq.Row) error {
	return func(row *sq.Row) error {
		r.URL = row.NullString(p.URL)
		r.Disabled = row.NullBool(p.DISABLED)
		r.RedirectURL = row.NullString(p.REDIRECT_URL)
		r.HandlerURL = row.NullString(p.HANDLER_URL)
		r.Content = row.NullString(p.CONTENT)
		r.ThemePath = row.NullString(p.THEME_PATH)
		r.Template = row.NullString(p.TEMPLATE)
		return nil
	}
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
	URLLogin           = "/pm-login"
	URLSuperadminLogin = "/pm-superadmin-login"
	URLDashboard       = "/pm-dashboard"
	URLEditPage        = "/pm-edit-page"   // GET,POST route=/url
	URLCreatePage      = "/pm-create-page" // GET,POST route=/url
	URLDeletePage      = "/pm-delete-page" // POST route=/url
)

const (
	cookieSuperadminSession       = "pm-superadmin-session"
	cookieSuperadminLoginRedirect = "pm-superadmin-login-redirect"
)

const (
	queryparamEditMode = "pm-edit"
	EditModeOff        = ""
	EditModeBasic      = "basic"
	EditModeAdvanced   = "advanced"

	queryparamJSON = "pm-json"
)
