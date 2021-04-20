package pagemanager

import (
	"errors"
)

// There's always a lighthouse. There's always a man. There's always a city.

const (
	URLLogin           = "/pm-login"
	URLLogout          = "/pm-logout"
	URLSuperadminLogin = "/pm-superadmin-login"
	URLDashboard       = "/pm-dashboard"
	URLViewPage        = "/pm-view-page"   // GET url=/url
	URLEditPage        = "/pm-edit-page"   // GET,POST url=/url
	URLCreatePage      = "/pm-create-page" // GET,POST url=/url
	URLDeletePage      = "/pm-delete-page" // POST url=/url
	// NOTE: after you delete, you aren't immediately redirected to the index
	// page. Instead you are redirected to the same page with all the page
	// details filled in, with a message saying "this page is deleted" together
	// with an option to undo the delete. Once you navigate away from that
	// page, the changes will be gone forever.
)

var (
	ErrBoxesNotInitialized     = errors.New("boxes not initialized")
	ErrInvalidLoginCredentials = errors.New("Invalid username/email or password")
)

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
	cookieSession        = "pm-session"
	cookieLoginRedirect  = "pm-login-redirect"
	cookieLogoutRedirect = "pm-logout-redirect"
)

const (
	queryparamEditMode = "pm-edit"
	EditModeOff        = ""
	EditModeBasic      = "basic"
	EditModeAdvanced   = "advanced"

	queryparamJSON = "pm-json"
)

const (
	PageTypeTemplate = "template"
	PageTypeContent  = "content"
	PageTypePlugin   = "plugin"
	PageTypeRedirect = "redirect"
	PageTypeDisabled = "disabled"
)
