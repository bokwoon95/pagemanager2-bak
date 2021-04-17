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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/encrypthash"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
	_ "github.com/mattn/go-sqlite3"
)

const superadminpassword = "lorem ipsum dolor sit amet"

type PageManager struct {
	publicBox           *encrypthash.Blackbox
	privateBox          *encrypthash.Blackbox
	privateBoxFlag      int32
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
		tables.NEW_AUTHZ_GROUPS(ctx, ""),
		tables.NEW_SESSIONS(ctx, ""),
		tables.NEW_LOCALES(ctx, ""),
	)
	if err != nil {
		return pm, erro.Wrap(err)
	}
	err = sq.EnsureTables(pm.superadminDB, "sqlite3",
		tables.NEW_SUPERADMIN(ctx, ""),
		tables.NEW_KEYS(ctx, ""),
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
	if atomic.LoadInt32(&pm.privateBoxFlag) == 0 {
		return nil, erro.Wrap(fmt.Errorf("lacking superadmin password"))
	}
	ctx := context.Background()
	KEYS := tables.NEW_KEYS(ctx, "")
	_, err = sq.Fetch(pm.superadminDB, sq.SQLite.From(KEYS).OrderBy(KEYS.ORDINAL_NUMBER), func(row *sq.Row) error {
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
	mux.HandleFunc("/pm-superadmin-login", pm.superadminLogin)
	mux.HandleFunc("/pm-test-encrypt", pm.testEncrypt)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/pm-themes/") ||
			strings.HasPrefix(r.URL.Path, "/pm-images/") ||
			strings.HasPrefix(r.URL.Path, "/pm-plugins/pagemanager/") {
			pm.serveFile(w, r, r.URL.Path)
			return
		}
		switch {
		case !*flagNoSetup:
			SUPERADMIN := tables.NEW_SUPERADMIN(r.Context(), "")
			superadminExists, err := sq.Exists(pm.superadminDB, sq.SQLite.From(SUPERADMIN))
			if err != nil {
				log.Println(erro.Wrap(err))
				break
			}
			if superadminExists {
				break
			}
			pm.superadminSetup(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (pm *PageManager) superadminDBL() sq.DB {
	return sq.NewDB(pm.superadminDB, sq.DefaultLogger(), sq.Lcompact)
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
			http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
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
		http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
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

func executeTemplates(w http.ResponseWriter, data interface{}, fsys fs.FS, file string, files ...string) error {
	b, err := fs.ReadFile(fsys, file)
	if err != nil {
		return erro.Wrap(err)
	}
	t, err := template.New(file).Funcs(map[string]interface{}{}).Parse(string(b)) // TODO: pm.funcmap()
	if err != nil {
		return erro.Wrap(err)
	}
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
	err = t.Execute(buf, data)
	if err != nil {
		return erro.Wrap(err)
	}
	buf.WriteTo(w)
	return nil
}

func (pm *PageManager) testEncrypt(w http.ResponseWriter, r *http.Request) {
	const secret = "secret"
	if atomic.LoadInt32(&pm.privateBoxFlag) == 0 {
		io.WriteString(w, "not yet loaded")
		return
	}
	// privateBox
	b, err := pm.privateBox.Base64Encrypt([]byte(secret))
	if err != nil {
		http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, "privateBox encrypted: "+string(b)+"\n")
	b, err = pm.privateBox.Base64Decrypt(b)
	if err != nil {
		http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, "privateBox decrypted: "+string(b)+"\n")
	b, err = pm.privateBox.Base64Hash([]byte(secret))
	if err != nil {
		http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, "privateBox hashedmsg: "+string(b)+"\n")
	b, err = pm.privateBox.Base64VerifyHash(b)
	if err != nil {
		http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, "privateBox msg: "+string(b)+"\n")
	// publicBox
	if pm.publicBox == nil {
		io.WriteString(w, "publicBox is nil")
		return
	}
	b, err = pm.publicBox.Base64Encrypt([]byte(secret))
	if err != nil {
		http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, "publicBox encrypted: "+string(b)+"\n")
	b, err = pm.publicBox.Base64Decrypt(b)
	if err != nil {
		http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, "publicBox decrypted: "+string(b)+"\n")
	b, err = pm.publicBox.Base64Hash([]byte(secret))
	if err != nil {
		http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, "publicBox hashedmsg: "+string(b)+"\n")
	b, err = pm.publicBox.Base64VerifyHash(b)
	if err != nil {
		http.Error(w, erro.Wrap(err).Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, "publicBox msg: "+string(b)+"\n")
	io.WriteString(w, "success\n")
}
