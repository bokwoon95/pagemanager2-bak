package erro

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ProjectDir string

func fmtCallerInfo(pc uintptr, file string, line int, _ bool) (filename string, linenr int, function string) {
	strs := strings.Split(runtime.FuncForPC(pc).Name(), "/")
	function = strs[len(strs)-1]
	filename = file
	if ProjectDir != "" {
		filename = strings.TrimPrefix(filename, filepath.Dir(ProjectDir))
		filename = strings.TrimPrefix(filename, string(os.PathSeparator))
	}
	return filename, line, function
}

func unwrapErr(err error) (errMsgs []string) {
	var prevMsg string
	for {
		if err.Error() != prevMsg {
			errMsgs = append(errMsgs, err.Error())
		}
		prevMsg = err.Error()
		err = errors.Unwrap(err)
		if err == nil {
			break
		}
	}
	var prevLen, currLen int
	for i := len(errMsgs) - 1; i >= 0; i-- {
		currLen = len(errMsgs[i])
		if prevLen == 0 {
			prevLen = currLen
			continue
		}
		errMsgs[i] = errMsgs[i][:currLen-prevLen]
		prevLen = currLen
	}
	return errMsgs
}

// Wrap will wrap an error and return a new error that is annotated with the
// function/file/linenumber of where Wrap() was called
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	filename, linenr, function := fmtCallerInfo(runtime.Caller(1))
	return fmt.Errorf("Error in %s:%d (%s) %w", filename, linenr, function, err)
}

// Dump will dump the formatted error string (with each error in its own line)
// into w io.Writer
func Dump(w io.Writer, err error) {
	if err == nil {
		io.WriteString(w, "<nil>")
		return
	}
	filename, linenr, function := fmtCallerInfo(runtime.Caller(1))
	err = fmt.Errorf("Error in %s:%d (%s) %w", filename, linenr, function, err)
	fmt.Fprintln(w, strings.Join(unwrapErr(err), "\n\n"))
}

// Sdump will return the formatted error string (with each error in its own
// line)
func Sdump(err error) string {
	if err == nil {
		return "<nil>"
	}
	filename, linenr, function := fmtCallerInfo(runtime.Caller(1))
	err = fmt.Errorf("Error in %s:%d (%s) %w", filename, linenr, function, err)
	return strings.Join(unwrapErr(err), "\n\n")
}
