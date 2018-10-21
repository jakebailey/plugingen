package example

import (
	"fmt"
	"io"
)

//go:generate go run .. -type=Thinger -subpkg=exampleplug .

type Thinger interface {
	fmt.Stringer
	DoNothing()
	Sum(...int) int
	Copy(io.Writer, io.Reader) (int64, error)
	ErrorToError(error) error
	Identity(interface{}) interface{}
	Replace(string, interface{ Replace(string) string }) string
}
