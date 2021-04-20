package pagemanager

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bokwoon95/pagemanager/encrypthash"
	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/hyforms"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
	_ "github.com/mattn/go-sqlite3"
)

const superadminpassword = "lorem ipsum dolor sit amet"

type PageManager struct {
	privateBoxFlag      int32
	privateBox          encrypthash.Box
	publicBox           encrypthash.Box
	themesMutex         *sync.RWMutex
	themes              map[string]theme
	fallbackAssetsIndex map[string]string // asset => theme name
	datafolder          string
	superadminfolder    string
	dataDB              *sql.DB
	superadminDB        *sql.DB
	innerEncryptionKey  []byte // key-stretched from user's low-entropy password
	innerMACKey         []byte // key-stretched from user's low-entropy password
	localesMutex        *sync.RWMutex
	locales             map[string]string
	plugins             map[string]map[string]http.Handler
}

func New() (*PageManager, error) {
	var err error
	pm := &PageManager{}
	pm.themesMutex = &sync.RWMutex{}
	pm.localesMutex = &sync.RWMutex{}
	pm.themes = make(map[string]theme)
	pm.datafolder, err = LocateDataFolder()
	if err != nil {
		return pm, erro.Wrap(err)
	}
	pm.superadminfolder, err = LocateSuperadminFolder(pm.datafolder)
	if err != nil {
		return pm, erro.Wrap(err)
	}
	pm.dataDB, err = sql.Open("sqlite3", filepath.Join(pm.datafolder, "database.sqlite3"+
		"?_journal_mode=WAL"+
		"&_synchronous=NORMAL"+
		"&_foreign_keys=on",
	))
	if err != nil {
		return pm, erro.Wrap(err)
	}
	pm.superadminDB, err = sql.Open("sqlite3", filepath.Join(pm.superadminfolder, "superadmin.sqlite3"+
		"?_journal_mode=WAL"+
		"&_synchronous=NORMAL"+
		"&_foreign_keys=on",
	))
	if err != nil {
		return pm, erro.Wrap(err)
	}
	ctx := context.Background()
	err = sq.EnsureTables(pm.dataDB, "sqlite3",
		tables.NEW_PAGES(ctx, ""),
		tables.NEW_PAGEDATA(ctx, ""),
		tables.NEW_USERS(ctx, ""),
		tables.NEW_ROLES(ctx, ""),
		tables.NEW_SESSIONS(ctx, ""),
		tables.NEW_LOCALES(ctx, ""),
	)
	if err != nil {
		return pm, erro.Wrap(err)
	}
	err = sq.EnsureTables(pm.superadminDB, "sqlite3",
		tables.NEW_SUPERADMIN(""),
		tables.NEW_KEYS(""),
	)
	if err != nil {
		return pm, erro.Wrap(err)
	}
	err = seedData(ctx, pm.dataDB)
	if err != nil {
		return pm, erro.Wrap(err)
	}
	pm.themes, pm.fallbackAssetsIndex, err = getThemes(pm.datafolder)
	if err != nil {
		return pm, erro.Wrap(err)
	}
	pm.locales, err = getLocales(ctx, pm.dataDB)
	if err != nil {
		return pm, erro.Wrap(err)
	}
	return pm, nil
}

func (pm *PageManager) getKeys() (keys [][]byte, err error) {
	if !pm.boxesInitialized() {
		return nil, erro.Wrap(fmt.Errorf("lacking superadmin password"))
	}
	KEYS := tables.NEW_KEYS("")
	_, err = sq.Fetch(pm.superadminDB, sq.SQLite.From(KEYS).OrderBy(KEYS.ORDER_NUM), func(row *sq.Row) error {
		key := row.Bytes(KEYS.KEY_CIPHERTEXT)
		return row.Accumulate(func() error {
			keys = append(keys, key)
			return nil
		})
	})
	if err != nil {
		return nil, erro.Wrap(err)
	}
	return keys, nil
}

func (pm *PageManager) PageManager(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", next)
	mux.HandleFunc(URLLogout, pm.logout)
	mux.HandleFunc(URLLogin, pm.login)
	mux.HandleFunc(URLSuperadminLogin, pm.superadminLogin)
	mux.HandleFunc(URLDashboard, pm.dashboard)
	mux.HandleFunc(URLCreatePage, pm.createPage)
	mux.HandleFunc("/pm-test-encrypt", pm.testEncrypt)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/pm-themes/") ||
			strings.HasPrefix(r.URL.Path, "/pm-images/") ||
			strings.HasPrefix(r.URL.Path, "/pm-plugins/pagemanager/") {
			pm.serveFile(w, r, r.URL.Path)
			return
		}
		page, localeCode, err := pm.getPage(r.Context(), r.URL.Path)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		r2 := &http.Request{} // r2 is like r, but with the localeCode stripped from the URL and injected into the request context
		*r2 = *r
		r2 = r2.WithContext(context.WithValue(r2.Context(), ctxKeyLocaleCode, localeCode))
		r2.URL = &url.URL{}
		*r2.URL = *r.URL
		r2.URL.Path = page.URL
		user, err := pm.getSession(w, r)
		if err == nil {
			r2 = r2.WithContext(context.WithValue(r2.Context(), ctxKeyUser, user))
		}
		switch {
		case !*flagNoSetup:
			SUPERADMIN := tables.NEW_SUPERADMIN("")
			superadminExists, _ := sq.Exists(pm.superadminDB, sq.SQLite.From(SUPERADMIN))
			if superadminExists {
				if !pm.boxesInitialized() && *flagPass != "" {
					err = pm.initializeBoxes([]byte(*flagPass))
					if err != nil {
						log.Printf("Incorrect password passed to -pm-pass")
					}
				}
				break
			}
			pm.superadminSetup(w, r2)
			return
		}
		switch page.PageType {
		case PageTypeTemplate:
			pm.serveTemplate(w, r2, page.ThemePath, page.TemplateName)
		case PageTypePlugin:
			if page.PluginName == "" || page.HandlerName == "" {
				pm.InternalServerError(w, r, erro.Wrap(fmt.Errorf("empty PluginName or HandlerName")))
				return
			}
			handler := pm.plugins[page.PluginName][page.HandlerName]
			if handler == nil {
				pm.InternalServerError(w, r, erro.Wrap(fmt.Errorf("handler not found for %s %s", page.PluginName, page.HandlerName)))
				return
			}
			handler.ServeHTTP(w, r2)
		case PageTypeContent:
			io.WriteString(w, page.Content)
		case PageTypeRedirect:
			Redirect(w, r, page.RedirectURL)
		case PageTypeDisabled:
			if page.Disabled {
				http.NotFound(w, r)
				return
			}
			fallthrough
		default:
			mux.ServeHTTP(w, r2)
		}
	})
}

func LocaleURL(r *http.Request, url string) string {
	path := url
	if url == "" {
		path = r.URL.Path
	}
	if !strings.HasPrefix(url, "/") {
		return path
	}
	localeCode, _ := r.Context().Value(ctxKeyLocaleCode).(string)
	if localeCode == "" {
		return path
	}
	return "/" + localeCode + path
}

func LocaleCode(r *http.Request) string {
	localeCode, _ := r.Context().Value(ctxKeyLocaleCode).(string)
	return localeCode
}

func (pm *PageManager) superadminDBL() sq.DB {
	return sq.NewDB(pm.superadminDB, sq.DefaultLogger(), sq.Lcompact)
}

func (pm *PageManager) dataDBL() sq.DB {
	return sq.NewDB(pm.dataDB, sq.DefaultLogger(), sq.Lcompact|sq.Lresults)
}

func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	if r.Method == "GET" {
		w.Header().Set("Cache-Control", "no-cache, private, max-age=0")
		w.Header().Set("Expires", time.Unix(0, 0).Format(http.TimeFormat))
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("X-Accel-Expires", "0")
	}
	http.Redirect(w, r, LocaleURL(r, url), http.StatusMovedPermanently)
}

func (pm *PageManager) RedirectToLogin(w http.ResponseWriter, r *http.Request) {
	USERS := tables.NEW_USERS(r.Context(), "u")
	usersExist, _ := sq.Exists(pm.dataDB, sq.SQLite.From(USERS).Where(USERS.USER_ID.NeInt(1)))
	if usersExist {
		Redirect(w, r, URLLogin)
		return
	}
	SUPERADMIN := tables.NEW_SUPERADMIN("sa")
	superadminExists, _ := sq.Exists(pm.superadminDB, sq.SQLite.
		From(SUPERADMIN).
		Where(
			SUPERADMIN.PASSWORD_HASH.IsNotNull(),
			SUPERADMIN.KEY_PARAMS.IsNotNull(),
		),
	)
	if !superadminExists {
		if *flagNoSetup {
			pm.InternalServerError(w, r, erro.Wrap(fmt.Errorf("missing superadmin")))
		} else {
			pm.superadminSetup(w, r)
		}
		return
	}
	Redirect(w, r, URLSuperadminLogin)
}

func (pm *PageManager) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	var f fs.File
	var err error
	if strings.HasPrefix(r.URL.Path, "/pm-plugins/pagemanager/") {
		path := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/pm-plugins/pagemanager/")
		f, err = pagemanagerFS.Open(path)
	}
	if strings.HasPrefix(r.URL.Path, "/pm-themes/") || strings.HasPrefix(r.URL.Path, "/pm-images/") {
		path := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		if strings.HasSuffix(path, "theme-config.js") || strings.HasSuffix(path, ".html") {
			http.NotFound(w, r)
			return
		}
		datafolderFS := os.DirFS(pm.datafolder)
		f, err = datafolderFS.Open(path)
		if errors.Is(err, os.ErrNotExist) {
			func() {
				missingFile := "/" + path
				pm.themesMutex.RLock()
				defer pm.themesMutex.RUnlock()
				themeName, ok := pm.fallbackAssetsIndex[missingFile]
				if !ok {
					return
				}
				theme, ok := pm.themes[themeName]
				if !ok {
					return
				}
				fallbackFile, ok := theme.fallbackAssets[missingFile]
				if !ok {
					return
				}
				f, err = datafolderFS.Open(strings.TrimPrefix(fallbackFile, "/"))
			}()
		}
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
		} else {
			pm.InternalServerError(w, r, erro.Wrap(err))
		}
		return
	}
	if f == nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		pm.InternalServerError(w, r, erro.Wrap(err))
		return
	}
	if info.IsDir() {
		http.NotFound(w, r)
		return
	}
	fseeker, ok := f.(io.ReadSeeker)
	if !ok {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, name, info.ModTime(), fseeker)
}

func (pm *PageManager) executeTemplates(w http.ResponseWriter, data interface{}, fsys fs.FS, file string, files ...string) error {
	b, err := fs.ReadFile(fsys, file)
	if err != nil {
		return erro.Wrap(err)
	}
	t, err := template.New(file).Funcs(pm.funcmap()).Parse(string(b))
	if err != nil {
		return erro.Wrap(err)
	}
	files = append([]string{"common.html"}, files...)
	for _, file := range files {
		b, err = fs.ReadFile(fsys, file)
		if err != nil {
			return erro.Wrap(err)
		}
		t, err = t.New(file).Parse(string(b))
		if err != nil {
			return erro.Wrap(err)
		}
	}
	buf := bufpool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufpool.Put(buf)
	}()
	err = t.ExecuteTemplate(buf, file, data)
	if err != nil {
		return erro.Wrap(err)
	}
	buf.WriteTo(w)
	return nil
}

func (pm *PageManager) getPage(ctx context.Context, path string) (page Page, localeCode string, err error) {
	elems := strings.SplitN(path, "/", 3) // because first character of path is always '/', we ignore the first element
	if len(elems) >= 2 {
		head := elems[1]
		pm.localesMutex.RLock()
		_, ok := pm.locales[head]
		pm.localesMutex.RUnlock()
		if ok {
			localeCode = head
			if len(elems) >= 3 {
				path = "/" + elems[2]
			} else {
				path = "/"
			}
		}
	}
	var negapath string
	if path == "/" {
		negapath = "/"
	} else if strings.HasSuffix(path, "/") {
		negapath = strings.TrimRight(path, "/")
	} else {
		negapath = path + "/"
	}
	PAGES := tables.NEW_PAGES(ctx, "p")
	_, err = sq.Fetch(pm.dataDB, sq.SQLite.
		From(PAGES).
		Where(PAGES.URL.In([]string{path, negapath})).
		OrderBy(sq.Case(PAGES.URL).When(path, 1).Else(2)).
		Limit(1),
		page.RowMapper(PAGES),
	)
	if err != nil {
		return page, localeCode, erro.Wrap(err)
	}
	if !page.Valid {
		page.URL = path
	}
	return page, localeCode, nil
}

func (pm *PageManager) testEncrypt(w http.ResponseWriter, r *http.Request) {
	user := pm.getUser(w, r)
	if user.Valid {
		fmt.Printf("testEncrypt user: %+v\n", user)
	}
	const secret = "secret"
	if !pm.boxesInitialized() {
		_ = hyforms.SetCookieValue(w, cookieLoginRedirect, LocaleURL(r, r.URL.Path), nil)
		Redirect(w, r, URLSuperadminLogin)
		return
	}
	// privateBox
	b, err := pm.privateBox.Base64Encrypt([]byte(secret))
	if err != nil {
		pm.InternalServerError(w, r, erro.Wrap(err))
		return
	}
	io.WriteString(w, "privateBox encrypted: "+string(b)+"\n")
	b, err = pm.privateBox.Base64Decrypt(b)
	if err != nil {
		pm.InternalServerError(w, r, erro.Wrap(err))
		return
	}
	io.WriteString(w, "privateBox decrypted: "+string(b)+"\n")
	b, err = pm.privateBox.Base64Hash([]byte(secret))
	if err != nil {
		pm.InternalServerError(w, r, erro.Wrap(err))
		return
	}
	io.WriteString(w, "privateBox hashedmsg: "+string(b)+"\n")
	b, err = pm.privateBox.Base64VerifyHash(b)
	if err != nil {
		pm.InternalServerError(w, r, erro.Wrap(err))
		return
	}
	io.WriteString(w, "privateBox msg: "+string(b)+"\n")
	// publicBox
	b, err = pm.publicBox.Base64Encrypt([]byte(secret))
	if err != nil {
		pm.InternalServerError(w, r, erro.Wrap(err))
		return
	}
	io.WriteString(w, "publicBox encrypted: "+string(b)+"\n")
	b, err = pm.publicBox.Base64Decrypt(b)
	if err != nil {
		pm.InternalServerError(w, r, erro.Wrap(err))
		return
	}
	io.WriteString(w, "publicBox decrypted: "+string(b)+"\n")
	b, err = pm.publicBox.Base64Hash([]byte(secret))
	if err != nil {
		pm.InternalServerError(w, r, erro.Wrap(err))
		return
	}
	io.WriteString(w, "publicBox hashedmsg: "+string(b)+"\n")
	b, err = pm.publicBox.Base64VerifyHash(b)
	if err != nil {
		pm.InternalServerError(w, r, erro.Wrap(err))
		return
	}
	io.WriteString(w, "publicBox msg: "+string(b)+"\n")
	io.WriteString(w, "success\n")
}
