package sharkerror

import (
	"strconv"
)

type Error struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func (e *Error) Error() string {
	return "code=" + strconv.Itoa(e.Code) + " msg=" + e.Msg
}

func (e *Error) Is(target error) bool {
	if target == nil {
		return false
	}
	t, ok := target.(*Error)
	return ok && e.Code == t.Code
}

func (e *Error) WithData(data any) *Error {
	return &Error{
		Code: e.Code,
		Msg:  e.Msg,
		Data: data,
	}
}

func (e *Error) WithErr(err error) *Error {
	return &Error{
		Code: e.Code,
		Msg:  e.Msg,
		Data: err.Error(),
	}
}

func (e *Error) WithMsg(msg string) *Error {
	return &Error{
		Code: e.Code,
		Msg:  msg,
		Data: e.Data,
	}
}

func New(code int, msg string) *Error {
	return &Error{
		Code: code,
		Msg:  msg,
	}
}
