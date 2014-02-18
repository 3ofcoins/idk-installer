package main

import "errors"
import "fmt"
import "path"
import "runtime"

type TaggedError struct {
	err error
	file string
	line int
	comment string
}

func (err *TaggedError) Error() (str string) {
	str = fmt.Sprintf("%s:%d: %v", err.file, err.line, err.err.Error())
	if err.comment != "" {
		str = fmt.Sprintf("%v (%v)", str, err.comment)
	}
	return
}

func NewErrf(format string, a ...interface{}) error {
	_, file, line, _ := runtime.Caller(0)
	return &TaggedError{errors.New(fmt.Sprintf(format, a...)), path.Base(file), line, ""}
}

func Err(err error) error {
	if err == nil { return nil }
	switch err.(type) {
	case *TaggedError: return err.(*TaggedError)
	default:
		_, file, line, _ := runtime.Caller(1)
		return &TaggedError{err, path.Base(file), line, ""}
	}
}

func Errf(err error, format string, a ...interface{}) error {
	if ( err == nil ) { return nil }
	_, file, line, _ := runtime.Caller(1)
	return &TaggedError{err, path.Base(file), line, fmt.Sprintf(format, a...)}
}
