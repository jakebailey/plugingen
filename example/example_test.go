package example_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/hashicorp/go-hclog"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/jakebailey/plugingen/example"
	"github.com/jakebailey/plugingen/example/exampleplug"
)

func TestThingerOneArgOneReturn(t *testing.T) {
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

	result := thinger.OneArgOneReturn(1234)
	if result != 1234 {
		t.Errorf("thinger.OneArgOneReturn(1234) = %v, want 1234", result)
	}
}

func BenchmarkThingerOneArgOneReturn(b *testing.B) {
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
		thinger.OneArgOneReturn(1234)
	}
}

func BenchmarkThingerOneArgOneReturnLocal(b *testing.B) {
	thinger := fakeThinger{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		thinger.OneArgOneReturn(1234)
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

type fakeThinger struct {
	example.Thinger
}

func (fakeThinger) OneArgOneReturn(i int) float32 {
	return float32(i)
}
