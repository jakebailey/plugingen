package generator

import (
	"bytes"
	"fmt"
	"go/types"
	"hash"
	"hash/fnv"
	"io"
	"log"
	"sort"

	"github.com/dave/jennifer/jen"
	"github.com/jakebailey/plugingen/analyzer"
	"github.com/jakebailey/plugingen/tojen"
	"github.com/jakebailey/plugingen/typesext"
)

const (
	paramsStructID  = "params"
	resultsStructID = "results"

	gopluginPath = "github.com/hashicorp/go-plugin"
	netrpcPath   = "net/rpc"
)

type Generator struct {
	allowError bool
	rpcPanic   bool

	file *jen.File

	ifaceNames map[*analyzer.Interface]string

	ifaceUnnamed      map[*types.Interface]string
	ifaceUnnamedCount int

	registerBasicError bool
}

func NewGenerator(allowError, rpcPanic bool, file *jen.File) *Generator {
	return &Generator{
		allowError:   allowError,
		rpcPanic:     rpcPanic,
		file:         file,
		ifaceNames:   map[*analyzer.Interface]string{},
		ifaceUnnamed: map[*types.Interface]string{},
	}
}

func (gen *Generator) Generate(ifaces []*analyzer.Interface) {
	h := fnv.New128a()
	imports := map[string]bool{}
	buf := &bytes.Buffer{}

	for _, iface := range ifaces {
		log.Println("generating plugin for", iface.Typ)
		gen.generateInterface(iface)
		gen.generatePlugin(iface)
		gen.generateRPC(iface)

		qf := func(pkg *types.Package) string {
			path := pkg.Path()
			imports[path] = true
			return path
		}

		buf.Reset()
		types.WriteType(buf, iface.Typ.Underlying(), qf)

		if _, err := buf.WriteTo(h); err != nil {
			log.Fatal(err)
		}
	}

	importsSlice := make([]string, 0, len(imports))
	for imp := range imports {
		importsSlice = append(importsSlice, imp)
	}
	sort.Strings(importsSlice)

	for _, imp := range importsSlice {
		if _, err := io.WriteString(h, imp); err != nil {
			log.Fatal(err)
		}
	}

	gen.generateHandshake(h)

	if gen.registerBasicError {
		gen.file.Func().Id("init").Params().BlockFunc(func(g *jen.Group) {
			g.Qual("encoding/gob", "Register").Call(jen.Op("&").Qual(gopluginPath, "BasicError").Values())
		})

		gen.registerBasicError = false
	}
}

func (gen *Generator) generateInterface(iface *analyzer.Interface) {
	if _, ok := iface.Typ.(*types.Named); ok {
		return
	}

	interfaceName, exists := gen.interfaceName(iface)
	if exists {
		return
	}

	typ := iface.Typ.Underlying()

	gen.file.Commentf("%s names an untyped interface. It should not be used directly.", interfaceName)
	gen.file.Type().Id(interfaceName).Add(tojen.Type(typ))
}

func (gen *Generator) generatePlugin(iface *analyzer.Interface) {
	interfaceName, _ := gen.interfaceName(iface)
	pluginName := gen.pluginName(iface)
	clientName := gen.clientName(iface)
	serverName := gen.serverName(iface)

	gen.file.Commentf("%s implements the Plugin interface for %s.", pluginName, interfaceName)

	gen.file.Type().Id(pluginName).Struct(
		jen.Id("impl").Add(tojen.Type(iface.Typ)),
	)

	gen.file.Func().Id("New" + pluginName).
		Params(jen.Id("impl").Add(tojen.Type(iface.Typ))).
		Op("*").Id(pluginName).
		Block(jen.Return(
			jen.Op("&").Id(pluginName).Values(jen.Dict{
				jen.Id("impl"): jen.Id("impl"),
			}),
		))

	gen.file.Var().Id("_").Qual(gopluginPath, "Plugin").Op("=").
		Parens(jen.Op("*").Id(pluginName)).Parens(jen.Nil()).
		Commentf("Compile-time check that %s is a Plugin.", pluginName).Line()

	gen.file.Comment("Server implements the Server method for the Plugin interface.")
	gen.file.Func().
		Params(jen.Id("p").Op("*").Id(pluginName)).
		Id("Server").
		Params(
			jen.Id("b").Op("*").Qual(gopluginPath, "MuxBroker"),
		).
		Params(
			jen.Interface(),
			jen.Error(),
		).
		Block(jen.Return(
			jen.Id("New"+serverName).Call(jen.Id("b"), jen.Id("p").Dot("impl")),
			jen.Nil(),
		))

	gen.file.Comment("Client implements the Client method for the Plugin interface.")
	gen.file.Func().
		Params(jen.Id("p").Op("*").Id(pluginName)).
		Id("Client").
		Params(
			jen.Id("b").Op("*").Qual(gopluginPath, "MuxBroker"),
			jen.Id("c").Op("*").Qual(netrpcPath, "Client"),
		).
		Params(
			jen.Interface(),
			jen.Error(),
		).
		Block(jen.Return(
			jen.Id("New"+clientName).Call(jen.Id("b"), jen.Id("c")),
			jen.Nil(),
		))
}

func (gen *Generator) generateRPC(iface *analyzer.Interface) {
	interfaceName, _ := gen.interfaceName(iface)

	clientName := gen.clientName(iface)
	gen.file.Commentf("%s implements %s via net/rpc.", clientName, interfaceName)
	gen.file.Type().Id(clientName).Struct(
		jen.Id("broker").Op("*").Qual(gopluginPath, "MuxBroker"),
		jen.Id("client").Op("*").Qual(netrpcPath, "Client"),
	)

	gen.file.Func().Id("New"+clientName).Params(
		jen.Id("b").Op("*").Qual(gopluginPath, "MuxBroker"),
		jen.Id("c").Op("*").Qual(netrpcPath, "Client"),
	).Op("*").Id(clientName).
		Block(jen.Return(jen.Op("&").Id(clientName).Values(jen.Dict{
			jen.Id("broker"): jen.Id("b"),
			jen.Id("client"): jen.Id("c"),
		})))

	gen.file.Var().Id("_").Add(tojen.Type(iface.Typ)).Op("=").
		Parens(jen.Op("*").Id(clientName)).Parens(jen.Nil())

	serverName := gen.serverName(iface)
	gen.file.Commentf("%s implements the net/rpc server for %s.", serverName, interfaceName)
	gen.file.Type().Id(serverName).Struct(
		jen.Id("broker").Op("*").Qual(gopluginPath, "MuxBroker"),
		jen.Id("impl").Add(tojen.Type(iface.Typ)),
	)

	gen.file.Func().Id("New"+serverName).Params(
		jen.Id("b").Op("*").Qual(gopluginPath, "MuxBroker"),
		jen.Id("impl").Add(tojen.Type(iface.Typ)),
	).Op("*").Id(serverName).
		Block(jen.Return(
			jen.Op("&").Id(serverName).Values(jen.Dict{
				jen.Id("broker"): jen.Id("b"),
				jen.Id("impl"):   jen.Id("impl"),
			})))

	for _, m := range iface.Methods {
		gen.generateRPCMethod(iface, m)
	}
}

func (gen *Generator) generateRPCMethod(iface *analyzer.Interface, m *analyzer.Method) {
	gen.generateRPCMethodStructs(iface, m)
	gen.generateRPCClientMethod(iface, m)
	gen.generateRPCServerMethod(iface, m)
}

func (gen *Generator) generateRPCMethodStructs(iface *analyzer.Interface, m *analyzer.Method) {
	paramsStructName := gen.paramsStructName(iface, m)
	resultsStructName := gen.resultsStructName(iface, m)

	if len(m.Params) != 0 {
		gen.file.Commentf("%s contains parameters for the %s function.", paramsStructName, m.Name)
		gen.file.Comment("It is exported for compatibility with net/rpc and should not be used directly.")
		gen.file.Type().Id(paramsStructName).StructFunc(func(g *jen.Group) {
			for i, param := range m.Params {
				if param.IFace != nil {
					g.Id(paramNameEx(i) + "ID").Uint32()
					continue
				}

				g.Id(paramNameEx(i)).Add(tojen.Type(param.Typ))
			}
		})
	}

	if len(m.Results) != 0 {
		gen.file.Commentf("%s contains results for the %s function.", resultsStructName, m.Name)
		gen.file.Comment("It is exported for compatibility with net/rpc and should not be used directly.")
		gen.file.Type().Id(resultsStructName).StructFunc(func(g *jen.Group) {
			for i, result := range m.Results {
				g.Id(resultNameEx(i)).Add(tojen.Type(result.Typ))
			}
		})
	}
}

func (gen *Generator) generateRPCClientMethod(iface *analyzer.Interface, m *analyzer.Method) {
	interfaceName, _ := gen.interfaceName(iface)
	clientName := gen.clientName(iface)
	paramsStructName := gen.paramsStructName(iface, m)
	resultsStructName := gen.resultsStructName(iface, m)

	gen.file.Commentf("%s implements %s for the %s interface.", m.Name, m.Name, interfaceName)
	gen.file.Func().
		Params(jen.Id("c").Op("*").Id(clientName)).
		Id(m.Name).
		ParamsFunc(func(g *jen.Group) {
			for i, param := range m.Params {
				if m.Variadic && i == len(m.Params)-1 {
					sl := param.Typ.(*types.Slice)
					g.Id(paramName(i)).Op("...").Add(tojen.Type(sl.Elem()))
				} else {
					g.Id(paramName(i)).Add(tojen.Type(param.Typ))
				}
			}
		}).
		ParamsFunc(func(g *jen.Group) {
			for _, result := range m.Results {
				g.Add(tojen.Type(result.Typ))
			}
		}).
		BlockFunc(func(g *jen.Group) {
			for i, param := range m.Params {
				if param.IFace == nil {
					continue
				}

				idName := paramName(i) + "id"
				g.Id(idName).Op(":=").
					Id("c").Dot("broker").Dot("NextId").Call()

				paramServerName := gen.serverName(param.IFace)

				g.Go().Id("c").Dot("broker").Dot("AcceptAndServe").Call(
					jen.Id(idName),
					jen.Id("New"+paramServerName).Call(
						jen.Id("c").Dot("broker"),
						jen.Id(paramName(i)),
					),
				)

				g.Line()
			}

			if len(m.Params) == 0 {
				g.Id(paramsStructID).Op(":=").New(jen.Interface())
			} else {
				g.Id(paramsStructID).Op(":=").
					Op("&").Id(paramsStructName).
					Values(jen.DictFunc(func(d jen.Dict) {
						for i, param := range m.Params {
							if !gen.allowError && typesext.IsError(param.Typ) {
								d[jen.Id(paramNameEx(i))] = jen.Qual(gopluginPath, "NewBasicError").
									Call(jen.Id(paramName(i)))
								continue
							}

							if param.IFace != nil {
								d[jen.Id(paramNameEx(i)+"ID")] = jen.Id(paramName(i) + "id")
								continue
							}

							d[jen.Id(paramNameEx(i))] = jen.Id(paramName(i))
						}
					}))
			}

			if len(m.Results) == 0 {
				g.Id(resultsStructID).Op(":=").New(jen.Interface())
			} else {
				g.Id(resultsStructID).Op(":=").Op("&").Id(resultsStructName).Values()
			}

			g.Line()

			var errFunc string
			if gen.rpcPanic {
				errFunc = "Fatalln"
			} else {
				errFunc = "Println"
			}

			g.If(
				jen.Id("err").Op(":=").Id("c").Dot("client").Dot("Call").Call(
					jen.Lit("Plugin."+m.Name),
					jen.Id(paramsStructID),
					jen.Id(resultsStructID),
				),
				jen.Id("err").Op("!=").Nil(),
			).Block(
				jen.Qual("log", errFunc).Call(
					jen.Lit(fmt.Sprintf("RPC call to %s.%s failed:", interfaceName, m.Name)),
					jen.Id("err").Dot("Error").Call(),
				),
			)

			if len(m.Results) != 0 {
				g.Line()
				g.ReturnFunc(func(g *jen.Group) {
					for i := range m.Results {
						g.Id(resultsStructID).Dot(resultNameEx(i))
					}
				})
			}
		})
}

func (gen *Generator) generateRPCServerMethod(iface *analyzer.Interface, m *analyzer.Method) {
	serverName := gen.serverName(iface)
	paramsStructName := gen.paramsStructName(iface, m)
	resultsStructName := gen.resultsStructName(iface, m)

	gen.file.Commentf("%s implements the server side of net/rpc calls to %s.", m.Name, m.Name)
	gen.file.Func().
		Params(jen.Id("s").Op("*").Id(serverName)).
		Id(m.Name).
		ParamsFunc(func(g *jen.Group) {
			if len(m.Params) == 0 {
				g.Id("_").Interface()
			} else {
				g.Id(paramsStructID).Op("*").Id(paramsStructName)
			}

			if len(m.Results) == 0 {
				g.Id("_").Op("*").Interface()
			} else {
				g.Id(resultsStructID).Op("*").Id(resultsStructName)
			}
		}).
		Params(jen.Error()).
		BlockFunc(func(g *jen.Group) {
			for i, param := range m.Params {
				if param.IFace == nil {
					continue
				}

				paramClientName := gen.clientName(param.IFace)
				idName := paramNameEx(i) + "ID"
				connName := paramName(i) + "conn"
				rpcName := paramName(i) + "RPCClient"
				clientName := paramName(i) + "client"

				g.List(jen.Id(connName), jen.Id("err")).Op(":=").
					Id("s").Dot("broker").Dot("Dial").Call(jen.Id("params").Dot(idName))

				g.If(jen.Id("err").Op("!=").Nil()).Block(jen.Return(jen.Id("err")))

				g.Id(rpcName).Op(":=").Qual(netrpcPath, "NewClient").Call(jen.Id(connName))
				g.Defer().Id(rpcName).Dot("Close").Call()

				g.Id(clientName).Op(":=").Id("New"+paramClientName).Call(
					jen.Id("s").Dot("broker"),
					jen.Id(rpcName),
				)

				g.Line()
			}

			line := g.Null()

			if len(m.Results) != 0 {
				line = g.ListFunc(func(g *jen.Group) {
					for i := range m.Results {
						g.Id(resultName(i))
					}
				}).Op(":=")
			}

			line.Id("s").
				Dot("impl").
				Dot(m.Name).
				ParamsFunc(func(g *jen.Group) {
					for i, param := range m.Params {
						if m.Variadic && i == len(m.Params)-1 {
							g.Id(paramsStructID).Dot(paramNameEx(i)).Op("...")
							continue
						}

						if param.IFace != nil {
							g.Id(paramName(i) + "client")
							continue
						}

						g.Id(paramsStructID).Dot(paramNameEx(i))
					}
				})

			g.Line()

			for i, result := range m.Results {
				if !gen.allowError && typesext.IsError(result.Typ) {
					g.If(jen.Id(resultName(i)).Op("==").Nil()).BlockFunc(func(g *jen.Group) {
						g.Id(resultsStructID).Dot(resultNameEx(i)).Op("=").Nil()
					}).Else().BlockFunc(func(g *jen.Group) {
						g.Id(resultsStructID).Dot(resultNameEx(i)).Op("=").
							Qual(gopluginPath, "NewBasicError").Call(jen.Id(resultName(i)))
					})
					gen.registerBasicError = true
					continue
				}

				g.Id(resultsStructID).Dot(resultNameEx(i)).Op("=").Id(resultName(i))
			}

			g.Line()
			g.Return(jen.Nil())
		})
}

func (gen *Generator) generateHandshake(h hash.Hash) {
	sum := fmt.Sprintf("%x", h.Sum(nil))

	gen.file.Comment("PluginHandshake is a plugin handshake generated from the input interfaces.")
	gen.file.Var().Id("PluginHandshake").Op("=").
		Qual(gopluginPath, "HandshakeConfig").Values(jen.Dict{
		jen.Id("ProtocolVersion"):  jen.Lit(1),
		jen.Id("MagicCookieKey"):   jen.Lit("PLUGINGEN_MAGIC_COOKIE_KEY"),
		jen.Id("MagicCookieValue"): jen.Lit(sum),
	})
}
