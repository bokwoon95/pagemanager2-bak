package pagemanager

import (
	"bytes"
	"context"
	"encoding/json"
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

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/encrypthash"
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
	p := tables.NEW_PAGES(ctx, "p")
	db = sq.NewDB(db, nil, sq.Linterpolate|sq.Lcaller)
	_, _, err := sq.Exec(db, sq.SQLite.DeleteFrom(p), sq.ErowsAffected)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_pages.content
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(p).
		Valuesx(func(col *sq.Column) error {
			col.SetString(p.URL, `/hello/`)
			col.SetString(p.CONTENT, `<h1>This is hello</h1>`)
			return nil
		}).
		OnConflict(p.URL).
		DoUpdateSet(sq.SetExcluded(p.CONTENT)),
		sq.ErowsAffected,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_pages.handler_url
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(p).
		Valuesx(func(col *sq.Column) error {
			col.SetString(p.URL, `/goodbye`)
			col.SetString(p.HANDLER_URL, `/`)
			return nil
		}).
		OnConflict(p.URL).
		DoUpdateSet(sq.SetExcluded(p.HANDLER_URL)),
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
		InsertInto(p).
		Valuesx(func(col *sq.Column) error {
			for _, t := range templates {
				col.SetString(p.URL, t.url)
				col.SetString(p.THEME_PATH, t.theme_path)
				col.SetString(p.TEMPLATE, t.template)
			}
			return nil
		}).
		OnConflict(p.URL).
		DoUpdateSet(sq.SetExcluded(p.THEME_PATH), sq.SetExcluded(p.TEMPLATE)),
		sq.ErowsAffected,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_sessions
	s := tables.NEW_SESSIONS(ctx, "s")
	var sessions = []struct {
		sessionhash string
		userid      int64
		sessiondata map[string]interface{}
	}{
		{"1234", 0, map[string]interface{}{"yeet": 1}},
	}
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(s).
		Valuesx(func(col *sq.Column) error {
			var b []byte
			for _, sess := range sessions {
				col.SetString(s.SESSION_HASH, sess.sessionhash)
				col.SetInt64(s.USER_ID, sess.userid)
				b, err = json.Marshal(sess.sessiondata)
				if err != nil {
					return erro.Wrap(err)
				}
				col.Set(s.SESSION_DATA, string(b))
			}
			return nil
		}).
		OnConflict().DoNothing(),
		sq.ErowsAffected,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	// pm_users, pm_authz_groups
	u, ag := tables.NEW_USERS(ctx, "u"), tables.NEW_AUTHZ_GROUPS(ctx, "ag")
	_, _, err = sq.Exec(db, sq.SQLite.DeleteFrom(u), sq.ErowsAffected)
	if err != nil {
		return erro.Wrap(err)
	}
	_, _, err = sq.Exec(db, sq.SQLite.DeleteFrom(ag), sq.ErowsAffected)
	if err != nil {
		return erro.Wrap(err)
	}
	var users = []struct {
		userid      int64
		publicid    string
		username    string
		authzgroups []string
	}{
		{1, "", "", []string{"pm-pagemanager"}},
	}
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(u).
		Valuesx(func(col *sq.Column) error {
			var b []byte
			for _, user := range users {
				col.SetInt64(u.USER_ID, user.userid)
				col.SetString(u.PUBLIC_USER_ID, user.publicid)
				col.SetString(u.USERNAME, user.username)
				b, err = json.Marshal(user.authzgroups)
				if err != nil {
					return erro.Wrap(err)
				}
				col.Set(u.AUTHZ_GROUPS, string(b))
			}
			return nil
		}),
		sq.ErowsAffected,
	)
	if err != nil {
		return erro.Wrap(err)
	}
	var groups = []struct {
		name string
		data map[string]interface{}
	}{
		{"pm-pagemanager", map[string]interface{}{"pm-page-perms": PageCreate | PageRead | PageUpdate | PageDelete}},
	}
	_, _, err = sq.Exec(db, sq.SQLite.
		InsertInto(ag).
		Valuesx(func(col *sq.Column) error {
			var b []byte
			for _, group := range groups {
				col.SetString(ag.NAME, group.name)
				b, err = json.Marshal(group.data)
				if err != nil {
					return erro.Wrap(err)
				}
				col.Set(ag.AUTHZ_DATA, string(b))
			}
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

const (
	PageCreate = 1 << iota
	PageRead
	PageUpdate
	PageDelete
)

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

func (pm *PageManager) hasSuperadminPassword() bool {
	return atomic.LoadInt32(&pm.privateBoxFlag) == 1
}

func (pm *PageManager) setSuperadminPassword(password []byte) error {
	ctx := context.Background()
	var passwordHash []byte
	var keyParams []byte
	SUPERADMIN := tables.NEW_SUPERADMIN(ctx, "")
	rowCount, err := sq.Fetch(pm.superadminDB, sq.SQLite.
		From(SUPERADMIN).
		Where(SUPERADMIN.ID.EqInt(1)),
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
		return fmt.Errorf("Invalid password")
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
