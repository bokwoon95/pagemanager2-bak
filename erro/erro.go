package erro

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ProjectDir string

const delim = "\x00->"

// Wrap will wrap an error and return a new error that is annotated with the
// function/file/linenumber of where Wrap() was called
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	pc, filename, linenr, _ := runtime.Caller(1)
	strs := strings.Split(runtime.FuncForPC(pc).Name(), "/")
	function := strs[len(strs)-1]
	if ProjectDir != "" {
		filename = strings.TrimPrefix(filename, filepath.Dir(ProjectDir))
		filename = strings.TrimPrefix(filename, string(os.PathSeparator))
	}
	return fmt.Errorf(delim+" Error in %s:%d (%s) %w", filename, linenr, function, err)
}

// Dump will dump the formatted error string (with each error in its own line)
// into w io.Writer
func Dump(w io.Writer, err error) {
	pc, filename, linenr, _ := runtime.Caller(1)
	strs := strings.Split(runtime.FuncForPC(pc).Name(), "/")
	function := strs[len(strs)-1]
	if ProjectDir != "" {
		filename = strings.TrimPrefix(filename, filepath.Dir(ProjectDir))
		filename = strings.TrimPrefix(filename, string(os.PathSeparator))
	}
	err = fmt.Errorf("Error in %s:%d (%s) %w", filename, linenr, function, err)
	fmtedErr := strings.Replace(err.Error(), " "+delim+" ", "\n\n", -1)
	fmt.Fprintln(w, fmtedErr)
}

// Sdump will return the formatted error string (with each error in its own
// line)
func Sdump(err error) string {
	pc, filename, linenr, _ := runtime.Caller(1)
	strs := strings.Split(runtime.FuncForPC(pc).Name(), "/")
	function := strs[len(strs)-1]
	if ProjectDir != "" {
		filename = strings.TrimPrefix(filename, filepath.Dir(ProjectDir))
		filename = strings.TrimPrefix(filename, string(os.PathSeparator))
	}
	err = fmt.Errorf("Error in %s:%d (%s) %w", filename, linenr, function, err)
	fmtedErr := strings.Replace(err.Error(), " "+delim+" ", "\n\n", -1)
	return fmtedErr
}
