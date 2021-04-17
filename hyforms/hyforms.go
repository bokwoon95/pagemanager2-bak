// Package hyforms is a form rendering and validation library for Go.
package hyforms

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/gob"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"time"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager/encrypthash"
	"github.com/bokwoon95/pagemanager/hy"
)

type ValidationErrMsgs struct {
	FormErrMsgs  []string
	InputErrMsgs map[string][]string
	Expires      time.Time
}

func (errMsgs ValidationErrMsgs) IsNonEmpty() bool {
	return len(errMsgs.FormErrMsgs) > 0 || len(errMsgs.InputErrMsgs) > 0
}

type Hyforms struct {
	box encrypthash.Box
}

var defaultHyforms = func() *Hyforms {
	key := make([]byte, 24)
	_, err := rand.Read(key)
	if err != nil {
		panic(err)
	}
	box, err := encrypthash.NewStaticKey(key)
	if err != nil {
		panic(err)
	}
	return &Hyforms{box: box}
}()

func (hyf *Hyforms) MarshalForm(s hy.Sanitizer, w http.ResponseWriter, r *http.Request, fn func(*Form)) (template.HTML, error) {
	form := &Form{
		request:      r,
		inputNames:   make(map[string]struct{}),
		inputErrMsgs: make(map[string][]string),
	}
	func() {
		c, _ := r.Cookie("hyforms.ValidationError")
		if c == nil {
			return
		}
		defer http.SetCookie(w, &http.Cookie{Name: "hyforms.ValidationError", MaxAge: -1})
		b, err := hyf.box.Base64VerifyHash([]byte(c.Value))
		if err != nil {
			return
		}
		validationErr := ValidationErrMsgs{}
		err = gob.NewDecoder(bytes.NewReader(b)).Decode(&validationErr)
		if err != nil {
			return
		}
		if time.Now().After(validationErr.Expires) {
			return
		}
		form.formErrMsgs = validationErr.FormErrMsgs
		form.inputErrMsgs = validationErr.InputErrMsgs
	}()
	fn(form)
	if len(form.marshalErrMsgs) > 0 {
		return "", erro.Wrap(fmt.Errorf("marshal errors %v", form.marshalErrMsgs))
	}
	output, err := hy.MarshalElement(s, form)
	if err != nil {
		return output, erro.Wrap(err)
	}
	return output, nil
}

func (hyf *Hyforms) UnmarshalForm(w http.ResponseWriter, r *http.Request, fn func(*Form)) ValidationErrMsgs {
	r.ParseForm()
	errMsgs := ValidationErrMsgs{InputErrMsgs: make(map[string][]string)}
	form := &Form{
		mode:         FormModeUnmarshal,
		request:      r,
		inputNames:   make(map[string]struct{}),
		inputErrMsgs: make(map[string][]string),
	}
	fn(form)
	if len(form.formErrMsgs) > 0 || len(form.inputErrMsgs) > 0 {
		errMsgs.FormErrMsgs = form.formErrMsgs
		for name, msgs := range form.inputErrMsgs {
			errMsgs.InputErrMsgs[name] = msgs
		}
	}
	return errMsgs
}

func (hyf *Hyforms) Redirect(w http.ResponseWriter, r *http.Request, url string, errMsgs ValidationErrMsgs) error {
	defer http.Redirect(w, r, url, http.StatusMovedPermanently)
	errMsgs.Expires = time.Now().Add(10 * time.Second)
	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode(errMsgs)
	if err != nil {
		return fmt.Errorf("%+v: failed gob encoding %s", errMsgs, err.Error())
	}
	value, err := hyf.box.Base64Hash(buf.Bytes())
	if err != nil {
		return erro.Wrap(err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "hyforms.ValidationError",
		Value:  string(value),
		MaxAge: 5,
	})
	return nil
}

func Redirect(w http.ResponseWriter, r *http.Request, url string, errMsgs ValidationErrMsgs) error {
	return defaultHyforms.Redirect(w, r, url, errMsgs)
}

func (hyf *Hyforms) CookieSet(w http.ResponseWriter, cookieName string, value interface{}, cookieTemplate *http.Cookie) error {
	buf := &bytes.Buffer{}
	switch value := value.(type) {
	case []byte:
		buf.Write(value)
	case string:
		buf.WriteString(value)
	default:
		err := gob.NewEncoder(buf).Encode(value)
		if err != nil {
			return erro.Wrap(err)
		}
	}
	b64HashedValue, err := hyf.box.Base64Hash(buf.Bytes())
	if err != nil {
		return erro.Wrap(err)
	}
	cookie := &http.Cookie{}
	if cookieTemplate != nil {
		*cookie = *cookieTemplate
	}
	cookie.Name = cookieName
	cookie.Value = string(b64HashedValue)
	http.SetCookie(w, cookie)
	return nil
}

func (hyf *Hyforms) CookieGet(r *http.Request, cookieName string, dest interface{}) error {
	c, err := r.Cookie(cookieName)
	if err != nil && !errors.Is(err, http.ErrNoCookie) {
		return erro.Wrap(err)
	}
	if c == nil {
		return nil
	}
	data, err := hyf.box.Base64VerifyHash([]byte(c.Value))
	if err != nil {
		return erro.Wrap(err)
	}
	switch dest := dest.(type) {
	case *[]byte:
		*dest = data
	case *string:
		*dest = string(data)
	default:
		err = gob.NewDecoder(bytes.NewReader(data)).Decode(dest)
		if err != nil {
			return erro.Wrap(err)
		}
	}
	return nil
}

func (hyf *Hyforms) CookiePop(w http.ResponseWriter, r *http.Request, cookieName string, dest interface{}) error {
	defer http.SetCookie(w, &http.Cookie{Name: cookieName, MaxAge: -1, Expires: time.Now().Add(-1 * time.Hour)})
	err := hyf.CookieGet(r, cookieName, dest)
	if err != nil {
		return erro.Wrap(err)
	}
	return nil
}

func MarshalForm(s hy.Sanitizer, w http.ResponseWriter, r *http.Request, fn func(*Form)) (template.HTML, error) {
	return defaultHyforms.MarshalForm(s, w, r, fn)
}

func UnmarshalForm(w http.ResponseWriter, r *http.Request, fn func(*Form)) ValidationErrMsgs {
	return defaultHyforms.UnmarshalForm(w, r, fn)
}

func CookieSet(w http.ResponseWriter, cookieName string, value interface{}, c *http.Cookie) error {
	return defaultHyforms.CookieSet(w, cookieName, value, c)
}

func CookieGet(r *http.Request, cookieName string, dest interface{}) error {
	return defaultHyforms.CookieGet(r, cookieName, dest)
}

func CookiePop(w http.ResponseWriter, r *http.Request, cookieName string, dest interface{}) error {
	return defaultHyforms.CookiePop(w, r, cookieName, dest)
}

func caller(skip int) (file string, line int, function string) {
	var pc [1]uintptr
	n := runtime.Callers(skip+2, pc[:])
	if n == 0 {
		return "???", 1, "???"
	}
	frame, _ := runtime.CallersFrames(pc[:n]).Next()
	return frame.File, frame.Line, frame.Function
}

func validateInput(f *Form, inputName string, value interface{}, validators []Validator) {
	if len(validators) == 0 {
		return
	}
	var stop bool
	var errMsg string
	ctx := f.request.Context()
	ctx = context.WithValue(ctx, ctxKeyName, inputName)
	for _, validator := range validators {
		stop, errMsg = validator(ctx, value)
		if errMsg != "" {
			f.inputErrMsgs[inputName] = append(f.inputErrMsgs[inputName], errMsg)
		}
		if stop {
			return
		}
	}
}
