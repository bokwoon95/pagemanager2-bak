package pagemanager

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
)

type PageData struct {
	Ctx        context.Context
	URL        string
	DataID     string
	LocaleCode string
	EditMode   string
	cssAssets  []Asset
	jsAssets   []Asset
	csp        map[string][]string
	json       map[string]interface{}
}

func NewPage() PageData {
	return PageData{
		csp:  make(map[string][]string),
		json: make(map[string]interface{}),
	}
}

func (pg PageData) CSS() (template.HTML, error) {
	var els hy.Elements
	for _, asset := range pg.cssAssets {
		if asset.Inline {
			continue // not handling inline assets for now
		}
		els = append(els, hy.H("link[rel=stylesheet]", hy.Attr{"href": asset.Path}))
	}
	return hy.MarshalElement(nil, els)
}

func (pg PageData) JS() (template.HTML, error) {
	var els hy.Elements
	jsonData, err := json.Marshal(pg.json)
	if err != nil {
		return "", erro.Wrap(err)
	}
	els = append(els, hy.H("script[data-pm-json][type=application/json]", nil, hy.Txt(jsonData)))
	for _, asset := range pg.jsAssets {
		if asset.Inline {
			continue
		}
		els = append(els, hy.H("script", hy.Attr{"src": asset.Path}))
	}
	return hy.MarshalElement(nil, els)
}

func (pg PageData) ContentSecurityPolicy() template.HTML {
	return template.HTML(fmt.Sprint(pg.csp))
}

type PageDataOption func(*PageData)

func pmLocale(localeCode string) PageDataOption {
	return func(pg *PageData) { pg.LocaleCode = localeCode }
}

func pmDataID(dataID string) PageDataOption {
	return func(pg *PageData) { pg.DataID = dataID }
}

type NullString struct {
	Valid bool
	Str   string
}

// Scan implements the Scanner interface.
func (ns *NullString) Scan(value interface{}) error {
	if value == nil {
		ns.Str, ns.Valid = "", false
		return nil
	}
	ns.Str = hy.Stringify(value)
	ns.Valid = true
	return nil
}

// Value implements the driver Valuer interface.
func (ns NullString) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return ns.String, nil
}

func (ns NullString) String() string {
	return ns.Str
}

func safeHTML(v interface{}) template.HTML {
	return template.HTML(hy.Stringify(v))
}

func jsonify(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func (pm *PageManager) pmGetValue(pg PageData, key string, opts ...PageDataOption) (NullString, error) {
	for _, opt := range opts {
		opt(&pg)
	}
	var ns NullString
	PAGEDATA := tables.NEW_PAGEDATA(pg.Ctx, "p")
	_, err := sq.FetchContext(pg.Ctx, pm.dataDB, sq.SQLite.
		From(PAGEDATA).
		Where(
			PAGEDATA.LOCALE_CODE.In([]string{pg.LocaleCode, ""}),
			PAGEDATA.DATA_ID.EqString(pg.DataID),
			PAGEDATA.KEY.EqString(key),
			PAGEDATA.ARRAY_INDEX.IsNull(),
		).
		OrderBy(sq.
			Case(PAGEDATA.LOCALE_CODE).
			When(pg.LocaleCode, 1).
			When("", 2),
		).
		Limit(1),
		func(row *sq.Row) error {
			row.ScanInto(&ns, PAGEDATA.VALUE)
			return nil
		},
	)
	if err != nil {
		return ns, erro.Wrap(err)
	}
	return ns, nil
}

func (pm *PageManager) pmGetRows(pg PageData, key string, opts ...PageDataOption) ([]interface{}, error) {
	for _, opt := range opts {
		opt(&pg)
	}
	PAGEDATA := tables.NEW_PAGEDATA(pg.Ctx, "p")
	exists, err := sq.ExistsContext(pg.Ctx, pm.dataDB, sq.SQLite.
		From(PAGEDATA).
		Where(
			PAGEDATA.LOCALE_CODE.EqString(pg.LocaleCode),
			PAGEDATA.DATA_ID.EqString(pg.DataID),
			PAGEDATA.KEY.EqString(key),
			PAGEDATA.ARRAY_INDEX.IsNotNull(),
		),
	)
	localeCode := pg.LocaleCode
	if !exists {
		localeCode = "" // default locale code
	}
	var values []interface{}
	var b []byte
	_, err = sq.FetchContext(pg.Ctx, pm.dataDB, sq.SQLite.
		From(PAGEDATA).
		Where(
			PAGEDATA.LOCALE_CODE.EqString(localeCode),
			PAGEDATA.DATA_ID.EqString(pg.DataID),
			PAGEDATA.KEY.EqString(key),
			PAGEDATA.ARRAY_INDEX.IsNotNull(),
		).
		OrderBy(PAGEDATA.ARRAY_INDEX),
		func(row *sq.Row) error {
			b = row.Bytes(PAGEDATA.VALUE)
			return row.Accumulate(func() error {
				value := make(map[string]interface{})
				err := json.Unmarshal(b, &value)
				if err != nil {
					values = append(values, string(b)) // couldn't unmarshal json, switching to string
				} else {
					values = append(values, value)
				}
				return nil
			})
		},
	)
	if err != nil {
		return values, erro.Wrap(err)
	}
	return values, nil
}

func (pm *PageManager) funcmap() map[string]interface{} {
	return map[string]interface{}{
		"jsonify":    jsonify,
		"safeHTML":   safeHTML,
		"pmGetValue": pm.pmGetValue,
		"pmGetRows":  pm.pmGetRows,
		"pmLocale":   pmLocale,
		"pmDataID":   pmDataID,
	}
}
