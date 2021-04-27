package tables

import (
	"context"

	"github.com/bokwoon95/pagemanager/sq"
)

type TenantIDKey struct{}

type PM_SUPERADMIN struct {
	sq.TableInfo
	ORDER_NUM     sq.NumberField `sq:"type=INTEGER misc=PRIMARY_KEY"`
	LOGIN_ID      sq.StringField
	PASSWORD_HASH sq.StringField
	KEY_PARAMS    sq.StringField
}

func NEW_SUPERADMIN(alias string) PM_SUPERADMIN {
	tbl := PM_SUPERADMIN{TableInfo: sq.TableInfo{Alias: alias}}
	tbl.TableInfo.Name = "pm_superadmin"
	_ = sq.ReflectTable(&tbl)
	return tbl
}

type PM_KEYS struct {
	sq.TableInfo
	ORDER_NUM      sq.NumberField
	KEY_CIPHERTEXT sq.StringField
	CREATED_AT     sq.TimeField
}

func NEW_KEYS(alias string) PM_KEYS {
	tbl := PM_KEYS{TableInfo: sq.TableInfo{Alias: alias}}
	tbl.TableInfo.Name = "pm_keys"
	_ = sq.ReflectTable(&tbl)
	return tbl
}

type PM_PAGES struct {
	sq.TableInfo
	URL       sq.StringField `sq:"type=TEXT misc=NOT_NULL,PRIMARY_KEY"`
	PAGE_TYPE sq.StringField
	// templates
	THEME_PATH    sq.StringField
	TEMPLATE_NAME sq.StringField
	// plugins
	PLUGIN_NAME  sq.StringField
	HANDLER_NAME sq.StringField
	// content body
	CONTENT sq.StringField
	// 301 Moved Permanently
	REDIRECT_URL sq.StringField
	// 404 Not Found
	HIDDEN sq.BooleanField
}

func NEW_PAGES(ctx context.Context, alias string) PM_PAGES {
	tbl := PM_PAGES{TableInfo: sq.TableInfo{Alias: alias}}
	if tenantID, ok := ctx.Value(TenantIDKey{}).(string); ok && tenantID != "" {
		tbl.TableInfo.Name = "pm_" + tenantID + "_pages"
	} else {
		tbl.TableInfo.Name = "pm_pages"
	}
	_ = sq.ReflectTable(&tbl)
	return tbl
}

type PM_PAGEDATA struct {
	sq.TableInfo
	LOCALE_CODE sq.StringField `sq:"misc=NOT_NULL"`
	DATA_ID     sq.StringField `sq:"misc=NOT_NULL"`
	KEY         sq.StringField `sq:"misc=NOT_NULL"`
	VALUE       sq.JSONField   `sq:"misc=NOT_NULL"`
	ARRAY_INDEX sq.NumberField `sq:""`
}

func NEW_PAGEDATA(ctx context.Context, alias string) PM_PAGEDATA {
	tbl := PM_PAGEDATA{TableInfo: sq.TableInfo{Alias: alias}}
	if tenantID, ok := ctx.Value(TenantIDKey{}).(string); ok && tenantID != "" {
		tbl.TableInfo.Name = "pm_" + tenantID + "_pagedata"
	} else {
		tbl.TableInfo.Name = "pm_pagedata"
	}
	_ = sq.ReflectTable(&tbl)
	return tbl
}

type PM_USERS struct {
	sq.TableInfo
	USER_ID        sq.NumberField `sq:"type=INTEGER misc=PRIMARY_KEY"`
	PUBLIC_USER_ID sq.StringField `sq:"type=TEXT misc=NOT_NULL,UNIQUE"`
	LOGIN_ID       sq.StringField
	PASSWORD_HASH  sq.StringField
	EMAIL          sq.StringField
	DISPLAYNAME    sq.StringField
	PERMISSIONS    sq.JSONField
	ROLES          sq.JSONField
	USER_DATA      sq.JSONField
}

func NEW_USERS(ctx context.Context, alias string) PM_USERS {
	tbl := PM_USERS{TableInfo: sq.TableInfo{Alias: alias}}
	if tenantID, ok := ctx.Value(TenantIDKey{}).(string); ok && tenantID != "" {
		tbl.TableInfo.Name = "pm_" + tenantID + "_users"
	} else {
		tbl.TableInfo.Name = "pm_users"
	}
	_ = sq.ReflectTable(&tbl)
	return tbl
}

type PM_PERMISSIONS struct {
	sq.TableInfo
	PERMISSION_NAME sq.StringField `sq:"type=TEXT misc=NOT_NULL,PRIMARY_KEY"`
	DESCRIPTION     sq.StringField
}

func NEW_PERMISSIONS(ctx context.Context, alias string) PM_PERMISSIONS {
	tbl := PM_PERMISSIONS{TableInfo: sq.TableInfo{Alias: alias}}
	if tenantID, ok := ctx.Value(TenantIDKey{}).(string); ok && tenantID != "" {
		tbl.TableInfo.Name = "pm_" + tenantID + "_permissions"
	} else {
		tbl.TableInfo.Name = "pm_permissions"
	}
	_ = sq.ReflectTable(&tbl)
	return tbl
}

type PM_ROLES struct {
	sq.TableInfo
	NAME        sq.StringField `sq:"type=TEXT misc=NOT_NULL,PRIMARY_KEY"`
	PERMISSIONS sq.JSONField
}

func NEW_ROLES(ctx context.Context, alias string) PM_ROLES {
	tbl := PM_ROLES{TableInfo: sq.TableInfo{Alias: alias}}
	if tenantID, ok := ctx.Value(TenantIDKey{}).(string); ok && tenantID != "" {
		tbl.TableInfo.Name = "pm_" + tenantID + "_roles"
	} else {
		tbl.TableInfo.Name = "pm_roles"
	}
	_ = sq.ReflectTable(&tbl)
	return tbl
}

type PM_SESSIONS struct {
	sq.TableInfo
	SESSION_HASH sq.StringField `sq:"type=TEXT misc=NOT_NULL,PRIMARY_KEY"`
	USER_ID      sq.NumberField `sq:"type=INTEGER misc=NOT_NULL"`
	CREATED_AT   sq.TimeField
	SESSION_DATA sq.JSONField
}

func NEW_SESSIONS(ctx context.Context, alias string) PM_SESSIONS {
	tbl := PM_SESSIONS{TableInfo: sq.TableInfo{Alias: alias}}
	if tenantID, ok := ctx.Value(TenantIDKey{}).(string); ok && tenantID != "" {
		tbl.TableInfo.Name = "pm_" + tenantID + "_sessions"
	} else {
		tbl.TableInfo.Name = "pm_sessions"
	}
	_ = sq.ReflectTable(&tbl)
	return tbl
}

type PM_LOCALES struct {
	sq.TableInfo
	LOCALE_CODE sq.StringField `sq:"type=TEXT misc=PRIMARY_KEY"`
	DESCRIPTION sq.StringField
}

func NEW_LOCALES(ctx context.Context, alias string) PM_LOCALES {
	tbl := PM_LOCALES{TableInfo: sq.TableInfo{Alias: alias}}
	if tenantID, ok := ctx.Value(TenantIDKey{}).(string); ok && tenantID != "" {
		tbl.TableInfo.Name = "pm_" + tenantID + "_locales"
	} else {
		tbl.TableInfo.Name = "pm_locales"
	}
	_ = sq.ReflectTable(&tbl)
	return tbl
}
