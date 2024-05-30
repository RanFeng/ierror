package ierror

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

const (
	gSplitStr = ": "
)

// CodeError的使用方式说明
// IError 自定义的错误类型
// Err  内层的错误
// Code 发生错误时的错误码
// Msg  错误码对应的详细信息
type IError struct {
	Err  error  `json:"err"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`

	pc    []uintptr `json:"-"`
	depth int       `json:"-"`
}

func (x *IError) Error() string {
	str := ""
	if x.Err != nil {
		str = fmt.Sprintf("%s%s", x.Err.Error(), gSplitStr)
	}
	return str + x.Msg
}

func (x *IError) C(skip int) *IError {
	pc := make([]uintptr, 32)
	n := runtime.Callers(skip, pc)
	x.pc, x.depth = pc[:n], n
	if e, ok := x.Err.(*IError); ok {
		e.depth -= x.depth
	}
	return x
}

// Unwrap是error类型的必要方法
func (x *IError) Unwrap() error {
	return x.Err
}

func Trace(err error) string {
	ge, ok := err.(*IError)
	if !ok {
		return err.Error()
	}
	str := ""
	if ge.Err != nil {
		_, ok = ge.Err.(*IError)
		str = Trace(ge.Err)
		if !ok {
			str = fmt.Sprintf("\nnot.found : %s\n\t/can/not/get/trace/info/:sorry", Trace(ge.Err))
		}
	}
	frames := runtime.CallersFrames(ge.pc[:ge.depth+1])
	f, ok := frames.Next()
	if ok {
		if ge.Code == 0 {
			str += pretty(&f, fmt.Sprintf("msg: %s", ge.Msg))
		} else {
			str += pretty(&f, fmt.Sprintf("msg: %s, code: %d", ge.Msg, ge.Code))
		}
	}
	f, ok = frames.Next()
	for ok {
		str += pretty(&f)
		f, ok = frames.Next()
	}
	return str
}

func Wrap(err error, msg string, skip ...int) *IError {
	var e = err
	// 不断解包，直到出现第一个CodeError，用于获取调用栈
	for {
		_, ok := e.(*IError)
		if ok {
			break
		}
		// 假设是errors生成的
		e1 := errors.Unwrap(e)
		if e1 == nil {
			break
		}
		e = e1
	}
	// 如果此处e为nil，
	// 表示e不是*IError类型也不是errors生成的
	// 此时无法解析出e的类型，直接将e包装起来即可
	ge := &IError{
		Code: 0,
		Err:  e,
		Msg:  msg,
	}
	if len(skip) == 0 {
		skip = []int{3}
	}
	return ge.C(skip[0])
}

// WrapIError 基于上层error封装出自定义错误
func WrapIError(err error, code int, msg string) *IError {
	ge := Wrap(err, msg, 4)
	ge.Code = code
	return ge
}

// NewIError 生成最底层的自定义错误
func NewIError(code int, msg string) *IError {
	ge := &IError{
		Code: code,
		Msg:  msg,
	}
	return ge.C(3)
}

// WrapWithFunc 将错误封装一层当前的函数名，并且返回新的错误
func WrapWithFunc(err error) error {
	ge := Wrap(err, "", 4)
	return ge
}

// Success 成功
const Success = 0

// ErrUnknown 未知错误
const ErrUnknown = -1

// GetErrorCode 尝试获取一个error对象的错误码
// 可能有三种情况：
// err == nil: 没有错误，返回0
// err 包含gmc.IError，返回CodeError中保存的code
// err 不包含gmc.IError，例如是mysql等组件直接返回的error，返回-1
func GetErrorCode(err error) int32 {
	var codeErr *IError
	// 处理err为nil的情况，注意err是interface，要用反射判断里面的value确实是nil
	if err == nil {
		return Success
	}
	rv := reflect.ValueOf(err)
	if rv.IsNil() {
		return Success
	}
	// 转换至CodeError并返回Code
	if ok := FirstAs(err, &codeErr); !ok {
		return ErrUnknown
	}
	return int32(codeErr.Code)
}

func FirstAs(err error, target **IError) bool {
	var e = err
	var last *IError
	for {
		if ok := errors.As(e, &last); !ok {
			if last != nil {
				*target = last
				return true
			}
			return false
		}
		*target = last
		if last.Code != 0 {
			return true
		}
		e = last.Err
	}
}

// ---------------------- 私有方法，只用于code error的 --------------------------
func pretty(frame *runtime.Frame, msg ...interface{}) string {
	//msg = append(msg, frame.Func, frame.Entry)
	return fmt.Sprintf("\n%s : %v\n\t%s:%d",
		frame.Function[strings.LastIndex(frame.Function, "/")+1:],
		msg,
		frame.File, frame.Line)
}
