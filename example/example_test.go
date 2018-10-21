package example_test

import (
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/jakebailey/plugingen/example"
	"github.com/jakebailey/plugingen/example/exampleplug"
)

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

func TestThingerSum(t *testing.T) {
	client := plugin.NewClient(&plugin.ClientConfig{
		Cmd:             helperProcess("thinger"),
		HandshakeConfig: exampleplug.PluginHandshake,
		Plugins: map[string]plugin.Plugin{
			"thinger": exampleplug.NewThingerPlugin(nil),
		},
		Logger: hclog.NewNullLogger(),
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	if err != nil {
		t.Fatal(err)
	}

	raw, err := rpcClient.Dispense("thinger")
	if err != nil {
		t.Fatal(err)
	}

	thinger := raw.(example.Thinger)

	got := thinger.Sum(1, 2, 3, 4)
	want := 1 + 2 + 3 + 4
	if got != want {
		t.Errorf("thinger.Sum(1, 2, 3, 4) = %v; want %v", got, want)
	}
}

func BenchmarkThingerSum(b *testing.B) {
	client := plugin.NewClient(&plugin.ClientConfig{
		Cmd:             helperProcess("thinger"),
		HandshakeConfig: exampleplug.PluginHandshake,
		Plugins: map[string]plugin.Plugin{
			"thinger": exampleplug.NewThingerPlugin(nil),
		},
		Logger: hclog.NewNullLogger(),
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	if err != nil {
		b.Fatal(err)
	}

	raw, err := rpcClient.Dispense("thinger")
	if err != nil {
		b.Fatal(err)
	}

	thinger := raw.(example.Thinger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		thinger.Sum(1, 2, 3, 4)
	}
}

func BenchmarkThingerSumLocal(b *testing.B) {
	thinger := fakeThinger{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		thinger.Sum(1, 2, 3, 4)
	}
}

const helperEnvVar = "PLUGINGEN_TEST_HELPER_PROCESS"

func helperProcess(args ...string) *exec.Cmd {
	if len(args) == 0 {
		panic("empty args")
	}

	args = append([]string{"-test.run=TestHelperProcess", "--"}, args...)

	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append([]string{helperEnvVar + "=1"}, os.Environ()...)
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnvVar) == "" {
		t.Skipf("%s not set", helperEnvVar)
	}

	args := os.Args[1:]
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) == 0 {
		t.Fatal("no args")
	}

	thinger := fakeThinger{}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: exampleplug.PluginHandshake,
		Plugins: map[string]plugin.Plugin{
			"thinger": exampleplug.NewThingerPlugin(thinger),
		},
	})
}
