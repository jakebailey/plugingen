package main

import (
	"fmt"
	"go/types"

	"github.com/jakebailey/plugingen/tojen"

	"github.com/dave/jennifer/jen"
)

const (
	paramsStructID  = "params"
	resultsStructID = "results"

	gopluginPath = "github.com/hashicorp/go-plugin"
	netrpcPath   = "net/rpc"
)

type Generator struct {
	file *jen.File

	ifaceNames map[*Interface]string

	ifaceUnnamed      map[*types.Interface]string
	ifaceUnnamedCount int
}

func NewGenerator(file *jen.File) *Generator {
	return &Generator{
		file:         file,
		ifaceNames:   map[*Interface]string{},
		ifaceUnnamed: map[*types.Interface]string{},
	}
}

func (gen *Generator) generate(iface *Interface) {
	gen.generateInterface(iface)
	gen.generatePlugin(iface)
	gen.generateRPC(iface)
}

func (gen *Generator) generateInterface(iface *Interface) {
	if _, ok := iface.typ.(*types.Named); ok {
		return
	}

	interfaceName, exists := gen.interfaceName(iface)
	if exists {
		return
	}

	typ := iface.typ.Underlying()

	gen.file.Commentf("%s names an untyped interface. It should not be used directly.", interfaceName)
	gen.file.Type().Id(interfaceName).Add(tojen.Type(typ))
}

func (gen *Generator) generatePlugin(iface *Interface) {
	interfaceName, _ := gen.interfaceName(iface)
	pluginName := gen.pluginName(iface)
	clientName := gen.clientName(iface)
	serverName := gen.serverName(iface)

	gen.file.Commentf("%s implements the Plugin interface for %s.", pluginName, interfaceName)

	gen.file.Type().Id(pluginName).Struct(
		jen.Id("impl").Add(tojen.Type(iface.typ)),
	)

	gen.file.Func().Id("New" + pluginName).
		Params(jen.Id("impl").Add(tojen.Type(iface.typ))).
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

func (gen *Generator) generateRPC(iface *Interface) {
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

	gen.file.Var().Id("_").Add(tojen.Type(iface.typ)).Op("=").
		Parens(jen.Op("*").Id(clientName)).Parens(jen.Nil())

	serverName := gen.serverName(iface)
	gen.file.Commentf("%s implements the net/rpc server for %s.", serverName, interfaceName)
	gen.file.Type().Id(serverName).Struct(
		jen.Id("broker").Op("*").Qual(gopluginPath, "MuxBroker"),
		jen.Id("impl").Add(tojen.Type(iface.typ)),
	)

	gen.file.Func().Id("New"+serverName).Params(
		jen.Id("b").Op("*").Qual(gopluginPath, "MuxBroker"),
		jen.Id("impl").Add(tojen.Type(iface.typ)),
	).Op("*").Id(serverName).
		Block(jen.Return(
			jen.Op("&").Id(serverName).Values(jen.Dict{
				jen.Id("broker"): jen.Id("b"),
				jen.Id("impl"):   jen.Id("impl"),
			})))

	for _, m := range iface.methods {
		gen.generateRPCMethod(iface, m)
	}
}

func (gen *Generator) generateRPCMethod(iface *Interface, m *Method) {
	gen.generateRPCMethodStructs(iface, m)
	gen.generateRPCClientMethod(iface, m)
	gen.generateRPCServerMethod(iface, m)
}

func (gen *Generator) generateRPCMethodStructs(iface *Interface, m *Method) {
	paramsStructName := gen.paramsStructName(iface, m)
	resultsStructName := gen.resultsStructName(iface, m)

	gen.file.Commentf("%s contains parameters for the %s function.", paramsStructName, m.name)
	gen.file.Comment("It is exported for compatibility with net/rpc and should not be used directly.")
	gen.file.Type().Id(paramsStructName).StructFunc(func(g *jen.Group) {
		for i, param := range m.params {
			if param.iface != nil {
				g.Id(paramNameEx(i) + "ID").Uint32()
				continue
			}

			g.Id(paramNameEx(i)).Add(tojen.Type(param.typ))
		}
	})

	gen.file.Commentf("%s contains results for the %s function.", resultsStructName, m.name)
	gen.file.Comment("It is exported for compatibility with net/rpc and should not be used directly.")
	gen.file.Type().Id(resultsStructName).StructFunc(func(g *jen.Group) {
		for i, result := range m.results {
			g.Id(resultNameEx(i)).Add(tojen.Type(result.typ))
		}
	})
}

func (gen *Generator) generateRPCClientMethod(iface *Interface, m *Method) {
	interfaceName, _ := gen.interfaceName(iface)
	clientName := gen.clientName(iface)
	paramsStructName := gen.paramsStructName(iface, m)
	resultsStructName := gen.resultsStructName(iface, m)

	gen.file.Func().
		Params(jen.Id("c").Op("*").Id(clientName)).
		Id(m.name).
		ParamsFunc(func(g *jen.Group) {
			for i, param := range m.params {
				if m.variadic && i == len(m.params)-1 {
					sl := param.typ.(*types.Slice)
					g.Id(paramName(i)).Op("...").Add(tojen.Type(sl.Elem()))
				} else {
					g.Id(paramName(i)).Add(tojen.Type(param.typ))
				}
			}
		}).
		ParamsFunc(func(g *jen.Group) {
			for _, result := range m.results {
				g.Add(tojen.Type(result.typ))
			}
		}).
		BlockFunc(func(g *jen.Group) {
			for i, param := range m.params {
				if param.iface == nil {
					continue
				}

				idName := paramName(i) + "id"
				g.Id(idName).Op(":=").
					Id("c").Dot("broker").Dot("NextId").Call()

				paramServerName := gen.serverName(param.iface)

				g.Go().Id("c").Dot("broker").Dot("AcceptAndServe").Call(
					jen.Id(idName),
					jen.Op("&").Id(paramServerName).Values(jen.Dict{
						jen.Id("impl"): jen.Id(paramName(i)),
					}),
				)

				g.Line()
			}

			g.Id(paramsStructID).Op(":=").
				Op("&").Id(paramsStructName).
				Values(jen.DictFunc(func(d jen.Dict) {
					for i, param := range m.params {
						if !*allowerror && isError(param.typ) {
							d[jen.Id(paramNameEx(i))] = jen.Qual(gopluginPath, "NewBasicError").
								Call(jen.Id(paramName(i)))
							continue
						}

						if types.IsInterface(param.typ) {
							d[jen.Id(paramNameEx(i)+"ID")] = jen.Id(paramName(i) + "id")
							continue
						}

						d[jen.Id(paramNameEx(i))] = jen.Id(paramName(i))
					}
				}))

			g.Id(resultsStructID).Op(":=").Op("&").Id(resultsStructName).Values()

			g.Line()

			g.If(
				jen.Id("err").Op(":=").Id("c").Dot("client").Dot("Call").Call(
					jen.Lit("Plugin."+m.name),
					jen.Id(paramsStructID),
					jen.Id(resultsStructID),
				),
				jen.Id("err").Op("!=").Nil(),
			).Block(
				jen.Qual("log", "Println").Call(
					jen.Lit(fmt.Sprintf("RPC call to %s.%s failed:", interfaceName, m.name)),
					jen.Id("err").Dot("Error").Call(),
				),
			)

			if len(m.results) != 0 {
				g.Line()
				g.ReturnFunc(func(g *jen.Group) {
					for i := range m.results {
						g.Id(resultsStructID).Dot(resultNameEx(i))
					}
				})
			}
		})
}

func (gen *Generator) generateRPCServerMethod(iface *Interface, m *Method) {
	serverName := gen.serverName(iface)
	paramsStructName := gen.paramsStructName(iface, m)
	resultsStructName := gen.resultsStructName(iface, m)

	gen.file.Commentf("%s implements the server side of net/rpc calls to %s.", m.name, m.name)
	gen.file.Func().
		Params(jen.Id("s").Op("*").Id(serverName)).
		Id(m.name).
		Params(
			jen.Id(paramsStructID).Op("*").Id(paramsStructName),
			jen.Id(resultsStructID).Op("*").Id(resultsStructName),
		).
		Params(jen.Error()).
		BlockFunc(func(g *jen.Group) {
			for i, param := range m.params {
				if param.iface == nil {
					continue
				}

				paramClientName := gen.clientName(param.iface)
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

			if len(m.results) != 0 {
				line = g.ListFunc(func(g *jen.Group) {
					for i := range m.results {
						g.Id(resultName(i))
					}
				}).Op(":=")
			}

			line.Id("s").
				Dot("impl").
				Dot(m.name).
				ParamsFunc(func(g *jen.Group) {
					for i, param := range m.params {
						if m.variadic && i == len(m.params)-1 {
							g.Id(paramsStructID).Dot(paramNameEx(i)).Op("...")
							continue
						}

						if param.iface != nil {
							g.Id(paramName(i) + "client")
							continue
						}

						g.Id(paramsStructID).Dot(paramNameEx(i))
					}
				})

			g.Line()

			for i, result := range m.results {
				if !*allowerror && isError(result.typ) {
					g.Id(resultsStructID).Dot(resultNameEx(i)).Op("=").
						Qual(gopluginPath, "NewBasicError").Call(jen.Id(resultName(i)))
					continue
				}

				g.Id(resultsStructID).Dot(resultNameEx(i)).Op("=").Id(resultName(i))
			}

			g.Line()
			g.Return(jen.Nil())
		})
}

func (gen *Generator) interfaceName(iface *Interface) (name string, exists bool) {
	if name, ok := gen.ifaceNames[iface]; ok {
		return name, true
	}

	if named, ok := iface.typ.(*types.Named); ok {
		name := named.Obj().Name()
		gen.ifaceNames[iface] = name
		return name, false
	}

	return gen.interfaceNameUnnamed(iface.typ)
}

func (gen *Generator) interfaceNameUnnamed(typ types.Type) (name string, exists bool) {
	t := typ.Underlying().(*types.Interface)

	if name, ok := gen.ifaceUnnamed[t]; ok {
		return name, true
	}

	name = fmt.Sprintf("Z_Interface%d", gen.ifaceUnnamedCount)
	gen.ifaceUnnamedCount++
	gen.ifaceUnnamed[t] = name
	return name, false
}

func (gen *Generator) pluginName(iface *Interface) string {
	name, _ := gen.interfaceName(iface)
	return name + "Plugin"
}

func (gen *Generator) clientName(iface *Interface) string {
	name, _ := gen.interfaceName(iface)
	return name + "RPCClient"
}

func (gen *Generator) serverName(iface *Interface) string {
	name, _ := gen.interfaceName(iface)
	return name + "RPCServer"
}

func (gen *Generator) paramsStructName(iface *Interface, m *Method) string {
	interfaceName, _ := gen.interfaceName(iface)
	return "Z_" + interfaceName + "_" + m.name + "Params"
}

func (gen *Generator) resultsStructName(iface *Interface, m *Method) string {
	interfaceName, _ := gen.interfaceName(iface)
	return "Z_" + interfaceName + "_" + m.name + "Results"
}
