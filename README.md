# plugingen

plugingen generates code to use arbitrary interfaces with hashicorp's
[go-plugin](https://github.com/hashicorp/go-plugin), including the use of `MuxBroker`
to support other interfaces as arguments.


## Caveats

plugingen comes with a few caveats:

- Values that aren't serializable by `encoding/gob` won't be handled correctly
    (ignoring interface arguments, which are brokered).
- Unless told otherwise, plugingen will wrap all errors in `plugin.BasicError`
    to ensure they are serialized.
- All types must be exported so that `net/rpc` will look at them. This means
    that the package where the generated code lives will fill with types for
    function parameters and return values. This is somewhat mitigated by
    prepending `Z_` to generated types, but it's still noisy.
- plugingen does not generate gRPC plugins. Maybe in the future.


## An example

Given the following interface:

```go
type Finder interface {
	Find(string, string) (int, bool)
}
```

This tool will generate the following (trimming out type declarations):

```go
// Find implements Find for the Finder interface.
func (c *FinderRPCClient) Find(p0 string, p1 string) (int, bool) {
	params := &Z_Finder_FindParams{
		P0: p0,
		P1: p1,
	}
	results := &Z_Finder_FindResults{}

	if err := c.client.Call("Plugin.Find", params, results); err != nil {
		log.Println("RPC call to Finder.Find failed:", err.Error())
	}

	return results.R0, results.R1
}

// Find implements the server side of net/rpc calls to Find.
func (s *FinderRPCServer) Find(params *Z_Finder_FindParams, results *Z_Finder_FindResults) error {
	r0, r1 := s.impl.Find(params.P0, params.P1)

	results.R0 = r0
	results.R1 = r1

	return nil
}
```

## A more complicated example

Take this more complicated interface:

```go
type Processor interface {
	Process(io.ReadCloser)
}
```

This interface requires a function which accepts an `io.ReadCloser`,
which cannot necessarily be serialized. plugingen produces this code to handle the interface:

```go
// Process implements Process for the Processor interface.
func (c *ProcessorRPCClient) Process(p0 io.ReadCloser) {
	p0id := c.broker.NextId()
	go c.broker.AcceptAndServe(p0id, NewReadCloserRPCServer(c.broker, p0))

	params := &Z_Processor_ProcessParams{P0ID: p0id}
	results := new(interface{})

	if err := c.client.Call("Plugin.Process", params, results); err != nil {
		log.Println("RPC call to Processor.Process failed:", err.Error())
	}
}

// Process implements the server side of net/rpc calls to Process.
func (s *ProcessorRPCServer) Process(params *Z_Processor_ProcessParams, _ *interface{}) error {
	p0conn, err := s.broker.Dial(params.P0ID)
	if err != nil {
		return err
	}
	p0RPCClient := rpc.NewClient(p0conn)
	defer p0RPCClient.Close()
	p0client := NewReadCloserRPCClient(s.broker, p0RPCClient)

	s.impl.Process(p0client)

	return nil
}
```

## TODOs

- Fix name collisions. If two interfaces are named the same thing, ignoring
	package names, then the output code will be broken. Interfaces coming from
	outside packages should include a prefix which summarizes their full name.
- Support variadic arguments of interfaces. This is technically doable just by
	inserting a for loop to broker each.
- Work out some quirks with printing type information. I use `Underlying()`
	quite a bit, which results in some cases where names get lost.
- Allow replacement of `net/rpc` and `hashicorp/go-plugin`. `net/rpc` is
	frozen, and doesn't have context support (and never will). The "solution"
	is to use gRPC, but that may pretty massively increase the footprint of
	this generator (to keep in tune with backwards compatibility), so it may
	be simpler to allow using a fork of `net/rpc` like
	[keegancsmith/rpc](https://github.com/keegancsmith/rpc) which allow for
	context. This would also require maintaining a fork of `go-plugin`.
- Testing. This is largely untested.
