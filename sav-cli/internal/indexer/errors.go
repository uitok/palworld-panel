package indexer

import "fmt"

const (
	CodeParserIncompatible = "parser_incompatible"
	CodeSavePathNotFound   = "save_path_not_found"
	CodeLevelSavNotFound   = "level_sav_not_found"
	CodeIndexFailed        = "index_failed"
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewError(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func ParserIncompatible(format string, args ...any) *Error {
	return NewError(CodeParserIncompatible, format, args...)
}
