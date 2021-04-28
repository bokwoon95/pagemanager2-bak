package pagemanager

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/bokwoon95/pagemanager/encrypthash"
	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/keyderiv"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
)

var (
	flagDatafolder       = flag.String("pm-datafolder", "", "")
	flagSuperadminFolder = flag.String("pm-superadmin", "", "")
	flagNoSetup          = flag.Bool("pm-no-setup", false, "")
	flagPass             = flag.String("pm-pass", "", "")
)

var bufpool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

var pagemanagerFS fs.FS

func init() {
	_, currentFile, _, _ := runtime.Caller(0)
	if pagemanagerFS == nil {
		pagemanagerFS = os.DirFS(filepath.Dir(currentFile))
	}
	erro.ProjectDir = filepath.Dir(currentFile)
}

func LocateDataFolder() (string, error) {
	const datafoldername = "pagemanager-data"
	cwd, err := os.Getwd()
	if err != nil {
		return "", erro.Wrap(err)
	}
	userhome, err := os.UserHomeDir()
	if err != nil {
		return "", erro.Wrap(err)
	}
	exePath, err := os.Executable()
	if err != nil {
		return "", erro.Wrap(err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", erro.Wrap(err)
	}
	exeDir := filepath.Dir(exePath)
	paths := []string{
		cwd,                                     // $CWD
		filepath.Join(cwd, datafoldername),      // $CWD/pagemanager-data
		filepath.Join(userhome, datafoldername), // $HOME/pagemanager-data
		exeDir,                                  // $EXE_DIR
		filepath.Join(exeDir, datafoldername),   // $EXE_DIR/pagemanager-data
	}
	if *flagDatafolder != "" {
		if strings.HasPrefix(*flagDatafolder, ".") {
			return cwd + (*flagDatafolder)[1:], nil
		}
		return *flagDatafolder, nil
	}
	for _, path := range paths {
		if filepath.Base(path) != datafoldername {
			continue
		}
		dir, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", erro.Wrap(err)
		}
		defer dir.Close()
		info, err := dir.Stat()
		if err != nil {
			return "", erro.Wrap(err)
		}
		if info.IsDir() {
			return path, nil
		}
	}
	defaultpath := filepath.Join(userhome, datafoldername)
	err = os.MkdirAll(defaultpath, 0775)
	if err != nil {
		return "", erro.Wrap(err)
	}
	err = os.MkdirAll(filepath.Join(defaultpath, "pm-themes"), 0775)
	if err != nil {
		return "", erro.Wrap(err)
	}
	err = os.MkdirAll(filepath.Join(defaultpath, "pm-images"), 0775)
	if err != nil {
		return "", erro.Wrap(err)
	}
	return defaultpath, nil
}

func LocateSuperadminFolder(datafolder string) (string, error) {
	const superadminfoldername = "pagemanager-superadmin"
	cwd, err := os.Getwd()
	if err != nil {
		return "", erro.Wrap(err)
	}
	userhome, err := os.UserHomeDir()
	if err != nil {
		return "", erro.Wrap(err)
	}
	exePath, err := os.Executable()
	if err != nil {
		return "", erro.Wrap(err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", erro.Wrap(err)
	}
	exeDir := filepath.Dir(exePath)
	paths := []string{
		cwd,                                      // $CWD
		filepath.Join(cwd, superadminfoldername), // $CWD/pagemanager-superadmin
		filepath.Join(userhome, superadminfoldername), // $HOME/pagemanager-superadmin
		exeDir, // $EXE_DIR
		filepath.Join(exeDir, superadminfoldername), // $EXE_DIR/pagemanager-superadmin
	}
	if *flagSuperadminFolder != "" {
		if strings.HasPrefix(*flagSuperadminFolder, ".") {
			return cwd + (*flagSuperadminFolder)[1:], nil
		}
		return *flagSuperadminFolder, nil
	}
	if !strings.HasSuffix(datafolder, string(os.PathSeparator)) {
		datafolder += string(os.PathSeparator)
	}
	for _, path := range paths {
		// superadminfolder must not be located inside the datafolder
		if strings.HasPrefix(path, datafolder) {
			continue
		}
		if filepath.Base(path) != superadminfoldername {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", erro.Wrap(err)
		}
		f.Close()
		return path, nil
	}
	defaultpath := filepath.Join(userhome, superadminfoldername)
	if strings.HasPrefix(defaultpath, datafolder) {
		return "", erro.Wrap(fmt.Errorf("superadminfolder defaultpath resides in the datafolder"))
	}
	err = os.MkdirAll(defaultpath, 0775)
	if err != nil {
		return "", erro.Wrap(err)
	}
	return defaultpath, nil
}

func seedData(ctx context.Context, db sq.Queryer) error {
	PAGES := tables.NEW_PAGES(ctx, "p")
	db = sq.NewDB(db, nil, sq.Linterpolate|sq.Lcaller)
	_, _, err := sq.Exec(db, sq.SQLite.DeleteFrom(PAGES), sq.ErowsAffected)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_pages.content
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(PAGES).
		Valuesx(func(col *sq.Column) error {
			col.SetString(PAGES.PAGE_TYPE, PageTypeContent)
			col.SetString(PAGES.URL, `/hello/`)
			col.SetString(PAGES.CONTENT, `<h1>This is hello</h1>`)
			return nil
		}).
		OnConflict(PAGES.URL).
		DoUpdateSet(sq.SetExcluded(PAGES.CONTENT)),
		sq.ErowsAffected,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_pages.theme_path, pm_pages.template
	var templates = []struct {
		url, theme_path, template string
	}{
		{"/posts", "plainsimple", "PostsIndex"},
	}
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(PAGES).
		Valuesx(func(col *sq.Column) error {
			for _, t := range templates {
				col.SetString(PAGES.PAGE_TYPE, PageTypeTemplate)
				col.SetString(PAGES.URL, t.url)
				col.SetString(PAGES.THEME_PATH, t.theme_path)
				col.SetString(PAGES.TEMPLATE_NAME, t.template)
			}
			return nil
		}).
		OnConflict(PAGES.URL).
		DoUpdateSet(sq.SetExcluded(PAGES.THEME_PATH), sq.SetExcluded(PAGES.TEMPLATE_NAME)),
		sq.ErowsAffected,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_users
	USERS := tables.NEW_USERS(ctx, "u")
	_, _, err = sq.Exec(db, sq.SQLite.DeleteFrom(USERS), sq.ErowsAffected)
	if err != nil {
		return erro.Wrap(err)
	}
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(USERS).
		Valuesx(func(col *sq.Column) error {
			col.SetInt64(USERS.USER_ID, 1)
			col.SetString(USERS.PUBLIC_USER_ID, "")
			col.SetString(USERS.LOGIN_ID, "")
			return nil
		}),
		sq.ErowsAffected,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_roles
	ROLES := tables.NEW_ROLES(ctx, "r")
	_, _, err = sq.Exec(db, sq.SQLite.DeleteFrom(ROLES), sq.ErowsAffected)
	if err != nil {
		return erro.Wrap(err)
	}
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(ROLES).
		Valuesx(func(col *sq.Column) error {
			col.SetString(ROLES.ROLE_NAME, roleSuperadmin)
			return nil
		}),
		sq.ErowsAffected,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_permissions
	PERMISSIONS := tables.NEW_PERMISSIONS(ctx, "p")
	_, _, err = sq.Exec(db, sq.SQLite.DeleteFrom(PERMISSIONS), sq.ErowsAffected)
	if err != nil {
		return erro.Wrap(err)
	}
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(PERMISSIONS).
		Valuesx(func(col *sq.Column) error {
			col.SetString(PERMISSIONS.PERMISSION_NAME, permissionAddPage)
			col.SetString(PERMISSIONS.PERMISSION_NAME, permissionViewPage)
			col.SetString(PERMISSIONS.PERMISSION_NAME, permissionChangePage)
			col.SetString(PERMISSIONS.PERMISSION_NAME, permissionDeletePage)
			return nil
		}),
		sq.ErowsAffected,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_user_roles
	USER_ROLES := tables.NEW_USER_ROLES(ctx, "ur")
	_, _, err = sq.Exec(db, sq.SQLite.DeleteFrom(USER_ROLES), sq.ErowsAffected)
	if err != nil {
		return erro.Wrap(err)
	}
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(USER_ROLES).
		Valuesx(func(col *sq.Column) error {
			col.SetInt64(USER_ROLES.USER_ID, 1)
			col.SetString(USER_ROLES.ROLE_NAME, roleSuperadmin)
			return nil
		}),
		sq.ErowsAffected,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_role_permissions
	ROLE_PERMISSIONS := tables.NEW_ROLE_PERMISSIONS(ctx, "rp")
	_, _, err = sq.Exec(db, sq.SQLite.DeleteFrom(ROLE_PERMISSIONS), sq.ErowsAffected)
	if err != nil {
		return erro.Wrap(err)
	}
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(ROLE_PERMISSIONS).
		Valuesx(func(col *sq.Column) error {
			// add
			col.SetString(ROLE_PERMISSIONS.ROLE_NAME, roleSuperadmin)
			col.SetString(ROLE_PERMISSIONS.PERMISSION_NAME, permissionAddPage)
			// view
			col.SetString(ROLE_PERMISSIONS.ROLE_NAME, roleSuperadmin)
			col.SetString(ROLE_PERMISSIONS.PERMISSION_NAME, permissionViewPage)
			// change
			col.SetString(ROLE_PERMISSIONS.ROLE_NAME, roleSuperadmin)
			col.SetString(ROLE_PERMISSIONS.PERMISSION_NAME, permissionChangePage)
			// delete
			col.SetString(ROLE_PERMISSIONS.ROLE_NAME, roleSuperadmin)
			col.SetString(ROLE_PERMISSIONS.PERMISSION_NAME, permissionDeletePage)
			return nil
		}),
		sq.ErowsAffected,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_locales
	l := tables.NEW_LOCALES(ctx, "l")
	_, _, err = sq.Exec(db, sq.SQLite.DeleteFrom(l), sq.ErowsAffected)
	if err != nil {
		return erro.Wrap(err)
	}
	var locales = []struct {
		code        string
		description string
	}{
		{"en", "English"},
		{"de", "German"},
	}
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(l).
		Valuesx(func(col *sq.Column) error {
			for _, locale := range locales {
				col.SetString(l.LOCALE_CODE, locale.code)
				col.SetString(l.DESCRIPTION, locale.description)
			}
			return nil
		}),
		sq.ErowsAffected,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_pagedata
	// pd := tables.NEW_PAGEDATA(ctx, "pd")
	// _, _, err = sq.Exec(db, sq.SQLite.
	// 	InsertInto(pd).
	// 	Valuesx(func(col *sq.Column) error {
	// 		col.SetString(pd.LOCALE_CODE, "")
	// 		col.SetString(pd.DATA_ID, "bokwoon95/plainsimple")
	// 		col.SetString(pd.KEY, "title")
	// 		col.Set(pd.VALUE, `BIG CHUNGUS`)
	// 		return nil
	// 	}),
	// 	sq.ErowsAffected,
	// )
	// if err != nil {
	// 	return erro.Wrap(err)
	// }
	return nil
}

func getLocales(ctx context.Context, db sq.Queryer) (map[string]string, error) {
	l := tables.NEW_LOCALES(ctx, "l")
	db = sq.NewDB(db, nil, sq.Linterpolate|sq.Lcaller|sq.Lresults)
	locales := make(map[string]string)
	_, err := sq.Fetch(db, sq.SQLite.From(l), func(row *sq.Row) error {
		localeCode := row.String(l.LOCALE_CODE)
		description := row.String(l.DESCRIPTION)
		return row.Accumulate(func() error {
			locales[localeCode] = description
			return nil
		})
	})
	if err != nil {
		return locales, erro.Wrap(err)
	}
	return locales, nil
}

func (pm *PageManager) boxesInitialized() bool {
	return atomic.LoadInt32(&pm.privateBoxFlag) == 1
}

func (pm *PageManager) initializeBoxes(password []byte) error {
	var passwordHash []byte
	var keyParams []byte
	SUPERADMIN := tables.NEW_SUPERADMIN("")
	rowCount, err := sq.Fetch(pm.superadminDB, sq.SQLite.
		From(SUPERADMIN).
		Where(SUPERADMIN.ORDER_NUM.EqInt(1)),
		func(row *sq.Row) error {
			passwordHash = row.Bytes(SUPERADMIN.PASSWORD_HASH)
			keyParams = row.Bytes(SUPERADMIN.KEY_PARAMS)
			return nil
		},
	)
	if err != nil {
		return erro.Wrap(err)
	}
	if rowCount == 0 {
		return fmt.Errorf("No superadmin found")
	}
	err = keyderiv.CompareHashAndPassword(passwordHash, password)
	if err != nil {
		return ErrInvalidLoginCredentials
	}
	var params keyderiv.Params
	err = params.UnmarshalBinary(keyParams)
	if err != nil {
		return erro.Wrap(err)
	}
	key := params.DeriveKey(password)
	pm.privateBox, err = encrypthash.NewStaticKey(key)
	if err != nil {
		return erro.Wrap(err)
	}
	pm.publicBox, err = encrypthash.NewRotatingKeys(pm.getKeys, pm.privateBox.Base64Decrypt)
	if err != nil {
		return erro.Wrap(err)
	}
	atomic.StoreInt32(&pm.privateBoxFlag, 1)
	return nil
}
