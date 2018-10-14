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


## An example

Given the following interface:

```go
type Finder interface {
	Find(string, string) (int, bool)
}
```

This tool will generate the following (trimming out type declarations):

```go
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
func (c *ProcessorRPCClient) Process(p0 io.ReadCloser) {
	p0id := c.broker.NextId()
	go c.broker.AcceptAndServe(p0id, &ReadCloserRPCServer{impl: p0})

	params := &Z_Processor_ProcessParams{P0ID: p0id}
	results := &Z_Processor_ProcessResults{}

	if err := c.client.Call("Plugin.Process", params, results); err != nil {
		log.Println("RPC call to Processor.Process failed:", err.Error())
	}
}

// Process implements the server side of net/rpc calls to Process.
func (s *ProcessorRPCServer) Process(params *Z_Processor_ProcessParams, results *Z_Processor_ProcessResults) error {
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