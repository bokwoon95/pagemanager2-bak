package sq

import (
	"bytes"
	"testing"

	"github.com/bokwoon95/pagemanager/testutil"
)

func Test_PredicateCases(t *testing.T) {
	type TT struct {
		excludedTableQualifiers []string
		wantQuery               string
		wantArgs                []interface{}
	}

	assertField := func(t *testing.T, f PredicateCases, tt TT) {
		is := testutil.New(t)
		var _ Field = f
		buf := &bytes.Buffer{}
		var args []interface{}
		err := f.AppendSQLExclude("", buf, &args, make(map[string]int), tt.excludedTableQualifiers)
		is.NoErr(err)
		is.Equal(tt.wantQuery, buf.String())
		is.Equal(f.alias, f.GetAlias())
		is.Equal("", f.GetName())
	}
	t.Run("empty", func(t *testing.T) {
		is := testutil.New(t)
		f := PredicateCases{}
		buf := &bytes.Buffer{}
		var args []interface{}
		err := f.AppendSQLExclude("", buf, &args, make(map[string]int), nil)
		is.True(err != nil)
	})
	t.Run("1 case", func(t *testing.T) {
		u := NEW_USERS("u")
		f := CaseWhen(u.USER_ID.IsNull(), 5)
		tt := TT{wantQuery: "CASE WHEN u.user_id IS NULL THEN ? END", wantArgs: []interface{}{5}}
		assertField(t, f, tt)
	})
	t.Run("2 cases", func(t *testing.T) {
		u := NEW_USERS("u")
		f := CaseWhen(u.USER_ID.IsNull(), 5).When(u.PASSWORD.EqString("abc"), u.EMAIL)
		tt := TT{
			wantQuery: "CASE WHEN u.user_id IS NULL THEN ? WHEN u.password = ? THEN u.email END",
			wantArgs:  []interface{}{5, "abc"},
		}
		assertField(t, f.As("alias"), tt)
	})
	t.Run("2 cases, fallback", func(t *testing.T) {
		u := NEW_USERS("u")
		f := CaseWhen(u.USER_ID.IsNull(), 5).When(u.PASSWORD.EqString("abc"), u.EMAIL).Else(6789)
		tt := TT{
			wantQuery: "CASE WHEN u.user_id IS NULL THEN ? WHEN u.password = ? THEN u.email ELSE ? END",
			wantArgs:  []interface{}{5, "abc", 6789},
		}
		assertField(t, f, tt)
	})
}

func Test_SimpleCases(t *testing.T) {
	type TT struct {
		excludedTableQualifiers []string
		wantQuery               string
		wantArgs                []interface{}
	}

	assertField := func(t *testing.T, f SimpleCases, tt TT) {
		is := testutil.New(t)
		var _ Field = f
		buf := &bytes.Buffer{}
		var args []interface{}
		err := f.AppendSQLExclude("", buf, &args, make(map[string]int), tt.excludedTableQualifiers)
		is.NoErr(err)
		is.Equal(tt.wantQuery, buf.String())
		is.Equal(f.alias, f.GetAlias())
		is.Equal("", f.GetName())
	}
	t.Run("empty", func(t *testing.T) {
		is := testutil.New(t)
		f := SimpleCases{}
		buf := &bytes.Buffer{}
		var args []interface{}
		err := f.AppendSQLExclude("", buf, &args, make(map[string]int), nil)
		is.True(err != nil)
	})
	t.Run("expression only", func(t *testing.T) {
		is := testutil.New(t)
		u := NEW_USERS("u")
		f := Case(u.USER_ID)
		buf := &bytes.Buffer{}
		var args []interface{}
		err := f.AppendSQLExclude("", buf, &args, make(map[string]int), nil)
		is.True(err != nil)
	})
	t.Run("expression, 1 case", func(t *testing.T) {
		u := NEW_USERS("u")
		f := Case(u.USER_ID).When(99, 97)
		tt := TT{
			wantQuery: "CASE u.user_id WHEN ? THEN ? END",
			wantArgs:  []interface{}{99, 97},
		}
		assertField(t, f, tt)
	})
	t.Run("expression, 2 cases", func(t *testing.T) {
		u := NEW_USERS("u")
		f := Case(u.USER_ID).When(99, 97).When(u.PASSWORD, u.EMAIL)
		tt := TT{
			wantQuery: "CASE u.user_id WHEN ? THEN ? WHEN u.password THEN u.email END",
			wantArgs:  []interface{}{99, 97},
		}
		assertField(t, f.As("alias"), tt)
	})
	t.Run("expression, 2 cases, fallback", func(t *testing.T) {
		u := NEW_USERS("u")
		f := Case(u.USER_ID).When(99, 97).When(u.PASSWORD, u.EMAIL).Else("abcde")
		tt := TT{
			wantQuery: "CASE u.user_id WHEN ? THEN ? WHEN u.password THEN u.email ELSE ? END",
			wantArgs:  []interface{}{99, 97, "abcde"},
		}
		assertField(t, f, tt)
	})
}
