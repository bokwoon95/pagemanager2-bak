package pagemanager

import "errors"

var (
	ErrBoxesNotInitialized = errors.New("boxes not initialized")
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
