package example_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/jakebailey/plugingen/example"
	"github.com/jakebailey/plugingen/example/exampleplug"
)

func TestString(t *testing.T) {
	thinger, cleanup := makeThinger(t)
	defer cleanup()

	got := thinger.String()
	want := "fakeThinger"
	if got != want {
		t.Errorf("thinger.String() = %v; want %v", got, want)
	}
}

func TestDoNothing(t *testing.T) {
	thinger, cleanup := makeThinger(t)
	defer cleanup()

	thinger.DoNothing()
}

func TestSum(t *testing.T) {
	thinger, cleanup := makeThinger(t)
	defer cleanup()

	got := thinger.Sum(1, 2, 3, 4)
	want := 1 + 2 + 3 + 4
	if got != want {
		t.Errorf("thinger.Sum(1, 2, 3, 4) = %v; want %v", got, want)
	}
}

func TestCopy(t *testing.T) {
	t.Skip("https://github.com/golang/go/issues/23340")

	thinger, cleanup := makeThinger(t)
	defer cleanup()

	s := "this is a test"
	r := strings.NewReader(s)
	buf := &bytes.Buffer{}

	n, err := thinger.Copy(buf, r)

	if n != int64(len(s)) {
		t.Errorf("thinger.Copy() copied %v bytes; want %v", n, len(s))
	}

	if err != nil {
		t.Errorf("thinger.Copy() returned error %v; want nil", err)
	}

	got := buf.String()
	if got != s {
		t.Errorf("thinger.Copy() copied `%v`; want `%v`", got, s)
	}
}

func TestErrorToError(t *testing.T) {
	t.Skip("https://github.com/golang/go/issues/23340")

	thinger, cleanup := makeThinger(t)
	defer cleanup()

	want := io.EOF
	got := thinger.ErrorToError(want)

	if got != want {
		t.Errorf("thinger.ErrorToError() = `%v`; want `%v`", got, want)
	}
}

func TestIdentity(t *testing.T) {
	thinger, cleanup := makeThinger(t)
	defer cleanup()

	tests := []interface{}{
		"foo",
		int(1234),
		float32(3.14159),
	}

	for _, want := range tests {
		got := thinger.Identity(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("thinger.Identity() = `%v`; want `%v`", got, want)
		}
	}
}

type replacer func(string) string

func (r replacer) Replace(s string) string {
	return r(s)
}

func TestReplace(t *testing.T) {
	thinger, cleanup := makeThinger(t)
	defer cleanup()

	r := replacer(func(s string) string { return s + "bar" })

	input := "foo"
	want := r.Replace(input)
	got := thinger.Replace(input, r)

	if got != want {
		t.Errorf("thinger.Replace() = `%v`; want `%v`", got, want)
	}
}

func BenchmarkSum(b *testing.B) {
	thinger, cleanup := makeThingerExternal(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		thinger.Sum(1, 2, 3, 4)
	}
}

func BenchmarkSumLocal(b *testing.B) {
	thinger := fakeThinger{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		thinger.Sum(1, 2, 3, 4)
	}
}

type fakeThinger struct{}

var _ example.Thinger = fakeThinger{}

func (fakeThinger) String() string {
	return "fakeThinger"
}

func (fakeThinger) DoNothing() {}

func (fakeThinger) Sum(vs ...int) int {
	sum := 0
	for _, v := range vs {
		sum += v
	}
	return sum
}

func (fakeThinger) Copy(w io.Writer, r io.Reader) (int64, error) {
	return io.Copy(w, r)
}

func (fakeThinger) ErrorToError(err error) error {
	return err
}

func (fakeThinger) Identity(v interface{}) interface{} {
	return v
}

func (fakeThinger) Replace(s string, i interface{ Replace(string) string }) string {
	return i.Replace(s)
}

var pluginSet = map[string]plugin.Plugin{
	"thinger": exampleplug.NewThingerPlugin(fakeThinger{}),
}

func makeThinger(t *testing.T) (example.Thinger, func()) {
	client, _ := plugin.TestPluginRPCConn(t, pluginSet, nil)

	raw, err := client.Dispense("thinger")
	if err != nil {
		t.Fatal(err)
	}

	return raw.(example.Thinger), func() {
		client.Close()
	}
}

type Fataler interface {
	Fatal(...interface{})
}

func makeThingerExternal(f Fataler) (example.Thinger, func()) {
	client := plugin.NewClient(&plugin.ClientConfig{
		Cmd:             helperProcess(),
		HandshakeConfig: exampleplug.PluginHandshake,
		Plugins:         pluginSet,
		Logger:          hclog.NewNullLogger(),
	})

	rpcClient, err := client.Client()
	if err != nil {
		f.Fatal(err)
	}

	raw, err := rpcClient.Dispense("thinger")
	if err != nil {
		f.Fatal(err)
	}

	return raw.(example.Thinger), func() {
		client.Kill()
	}
}

const helperEnvVar = "PLUGINGEN_TEST_HELPER_PROCESS"

func helperProcess() *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd.Env = append([]string{helperEnvVar + "=1"}, os.Environ()...)
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnvVar) == "" {
		t.Skipf("%s not set", helperEnvVar)
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: exampleplug.PluginHandshake,
		Plugins:         pluginSet,
	})
}
