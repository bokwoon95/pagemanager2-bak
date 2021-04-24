package pagemanager

import (
	"errors"
)

// There's always a lighthouse. There's always a man. There's always a city.

const (
	URLLogout          = "/pm-logout"
	URLLogin           = "/pm-login"
	URLSuperadminLogin = "/pm-superadmin-login"
	URLDashboard       = "/pm-dashboard"
	URLCreatePage      = "/pm-create-page" // GET,POST url=/url
	URLViewPage        = "/pm-view-page"   // GET url=/url
	URLEditPage        = "/pm-edit-page"   // GET,POST url=/url
	URLDeletePage      = "/pm-delete-page" // POST url=/url
	// NOTE: after you delete, you aren't immediately redirected to the index
	// page. Instead you are redirected to the same page with all the page
	// details filled in, with a message saying "this page is deleted" together
	// with an option to undo the delete. Once you navigate away from that
	// page, the changes will be gone forever.
	URLConsole   = "/pm-console"
	URLAnalytics = "/pm-analytics"
)

// superadminURLs are the URLs where a superadmin account is needed, and the
// user is directed to create a superadmin account if none exists.
var superadminURLs = map[string]struct{}{
	URLLogout: {}, URLLogin: {}, URLSuperadminLogin: {}, URLDashboard: {},
	URLCreatePage: {}, URLViewPage: {}, URLEditPage: {}, URLDeletePage: {},
	URLConsole: {}, URLAnalytics: {},
}

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
	roleSuperadmin = "pagemanager:superadmin"
)

const (
	permissionPagePerms = "pagemanager:page-perms"
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
