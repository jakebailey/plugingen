package testdata

import (
	"bytes"
	"fmt"
	"io"
)

//go:generate go run .. -type=Thinger -subpkg=plugingen .

type Thinger interface {
	Simple()
	TwoReturns() (int, bool)
	ErrReturn() (int, error)
	OneArg(int)
	TwoArgs(*int, string)
	OneReturn() int
	OneArgOneReturn(int) float32
	TwoArgsTwoReturns(byte, []byte) (int, string)
	DoubleParams(a, b int)
	DoubleReturns() (a, b string)
	Variadic(string, ...int)
	Imported(bytes.Buffer)
	ImportedInterface(io.Reader, io.Writer)
	InterfaceParam(stringer fmt.Stringer)
	ErrorToError(error) error
	Literal(interface {
		Replace(string) string
	})
	EmptyInterface(interface{}) interface{}
	fmt.Stringer
}
