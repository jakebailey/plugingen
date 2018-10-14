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

func (g *Generator) generate(iface *Interface) {
	g.generateInterface(iface)
	g.generatePlugin(iface)
	g.generateRPC(iface)
}

func (g *Generator) generateInterface(iface *Interface) {
	if _, ok := iface.typ.(*types.Named); ok {
		return
	}

	interfaceName, exists := g.interfaceName(iface)
	if exists {
		return
	}

	typ := iface.typ.Underlying()

	g.file.Commentf("%s names an untyped interface. It should not be used directly.", interfaceName)
	g.file.Type().Id(interfaceName).Add(tojen.Type(typ))
}

func (g *Generator) generatePlugin(iface *Interface) {
	interfaceName, _ := g.interfaceName(iface)
	pluginName := g.pluginName(iface)
	clientName := g.clientName(iface)
	serverName := g.serverName(iface)

	g.file.Commentf("%s implements the Plugin interface for %s.", pluginName, interfaceName)

	g.file.Type().Id(pluginName).Struct(
		jen.Id("Impl").Add(tojen.Type(iface.typ)),
	)

	g.file.Var().Id("_").Qual(gopluginPath, "Plugin").Op("=").
		Parens(jen.Op("*").Id(pluginName)).Parens(jen.Nil()).
		Commentf("Compile-time check that %s is a Plugin.", pluginName).Line()

	g.file.Comment("Server implements the Server method for the Plugin interface.")
	g.file.Func().
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
			jen.Op("&").Id(serverName).Values(jen.Dict{
				jen.Id("Impl"): jen.Id("p").Dot("Impl"),
			}),
			jen.Nil(),
		))

	g.file.Comment("Client implements the Client method for the Plugin interface.")
	g.file.Func().
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
			jen.Op("&").Id(clientName).Values(jen.Dict{
				jen.Id("broker"): jen.Id("b"),
				jen.Id("client"): jen.Id("c"),
			}),
			jen.Nil(),
		))
}

func (g *Generator) generateRPC(iface *Interface) {
	interfaceName, _ := g.interfaceName(iface)

	clientName := g.clientName(iface)
	g.file.Commentf("%s implements %s via net/rpc.", clientName, interfaceName)
	g.file.Type().Id(clientName).Struct(
		jen.Id("broker").Op("*").Qual(gopluginPath, "MuxBroker"),
		jen.Id("client").Op("*").Qual(netrpcPath, "Client"),
	)

	g.file.Var().Id("_").Add(tojen.Type(iface.typ)).Op("=").
		Parens(jen.Op("*").Id(clientName)).Parens(jen.Nil())

	serverName := g.serverName(iface)
	g.file.Commentf("%s implements the net/rpc server for %s", serverName, interfaceName)
	g.file.Type().Id(serverName).Struct(
		jen.Id("Impl").Add(tojen.Type(iface.typ)),
	)

	for _, m := range iface.methods {
		g.generateRPCMethod(iface, m)
	}
}

func (g *Generator) generateRPCMethod(iface *Interface, m *Method) {
	g.generateRPCMethodStructs(iface, m)
	g.generateRPCClientMethod(iface, m)
	g.generateRPCServerMethod(iface, m)
}

func (g *Generator) generateRPCMethodStructs(iface *Interface, m *Method) {
	paramsStructName := g.paramsStructName(iface, m)
	resultsStructName := g.resultsStructName(iface, m)

	g.file.Commentf("%s contains parameters for the %s function.", paramsStructName, m.name)
	g.file.Comment("It is exported for compatibility with net/rpc and should not be used directly.")
	g.file.Type().Id(paramsStructName).StructFunc(func(g *jen.Group) {
		for i, param := range m.params {
			g.Id(paramNameEx(i)).Add(tojen.Type(param.typ))
		}
	})

	g.file.Commentf("%s contains results for the %s function.", resultsStructName, m.name)
	g.file.Comment("It is exported for compatibility with net/rpc and should not be used directly.")
	g.file.Type().Id(resultsStructName).StructFunc(func(g *jen.Group) {
		for i, result := range m.results {
			g.Id(resultNameEx(i)).Add(tojen.Type(result.typ))
		}
	})
}

func (g *Generator) generateRPCClientMethod(iface *Interface, m *Method) {
	interfaceName, _ := g.interfaceName(iface)
	clientName := g.clientName(iface)
	paramsStructName := g.paramsStructName(iface, m)
	resultsStructName := g.resultsStructName(iface, m)

	g.file.Func().
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
			g.Id(paramsStructID).Op(":=").
				Op("&").Id(paramsStructName).
				Values(jen.DictFunc(func(d jen.Dict) {
					for i, param := range m.params {
						if !*allowerror && isError(param.typ) {
							d[jen.Id(paramNameEx(i))] = jen.Qual(gopluginPath, "NewBasicError").
								Call(jen.Id(paramName(i)))
							continue
						}

						d[jen.Id(paramNameEx(i))] = jen.Id(paramName(i))
					}
				}))

			g.Id(resultsStructID).Op(":=").Op("&").Id(resultsStructName).Values()

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
				g.ReturnFunc(func(g *jen.Group) {
					for i := range m.results {
						g.Id(resultsStructID).Dot(resultNameEx(i))
					}
				})
			}
		})
}

func (g *Generator) generateRPCServerMethod(iface *Interface, m *Method) {
	serverName := g.serverName(iface)
	paramsStructName := g.paramsStructName(iface, m)
	resultsStructName := g.resultsStructName(iface, m)

	g.file.Commentf("%s implements the server side of net/rpc calls to %s.", m.name, m.name)
	g.file.Func().
		Params(jen.Id("s").Op("*").Id(serverName)).
		Id(m.name).
		Params(
			jen.Id(paramsStructID).Op("*").Id(paramsStructName),
			jen.Id(resultsStructID).Op("*").Id(resultsStructName),
		).
		Params(jen.Error()).
		BlockFunc(func(g *jen.Group) {
			line := g.Null()

			if len(m.results) != 0 {
				line = g.ListFunc(func(g *jen.Group) {
					for i := range m.results {
						g.Id(resultName(i))
					}
				}).Op(":=")
			}

			line.Id("s").
				Dot("Impl").
				Dot(m.name).
				ParamsFunc(func(g *jen.Group) {
					for i := range m.params {
						if m.variadic && i == len(m.params)-1 {
							g.Id(paramsStructID).Dot(paramNameEx(i)).Op("...")
						} else {
							g.Id(paramsStructID).Dot(paramNameEx(i))
						}
					}
				})

			for i, result := range m.results {
				if !*allowerror && isError(result.typ) {
					g.Id(resultsStructID).Dot(resultNameEx(i)).Op("=").
						Qual(gopluginPath, "NewBasicError").Call(jen.Id(resultName(i)))
					continue
				}

				g.Id(resultsStructID).Dot(resultNameEx(i)).Op("=").Id(resultName(i))
			}

			g.Return(jen.Nil())
		})
}

func (g *Generator) interfaceName(iface *Interface) (name string, exists bool) {
	if name, ok := g.ifaceNames[iface]; ok {
		return name, true
	}

	if named, ok := iface.typ.(*types.Named); ok {
		name := named.Obj().Name()
		g.ifaceNames[iface] = name
		return name, false
	}

	return g.interfaceNameUnnamed(iface.typ)
}

func (g *Generator) interfaceNameUnnamed(typ types.Type) (name string, exists bool) {
	t := typ.Underlying().(*types.Interface)

	if name, ok := g.ifaceUnnamed[t]; ok {
		return name, true
	}

	name = fmt.Sprintf("Z_Interface%d", g.ifaceUnnamedCount)
	g.ifaceUnnamedCount++
	g.ifaceUnnamed[t] = name
	return name, false
}

func (g *Generator) pluginName(iface *Interface) string {
	name, _ := g.interfaceName(iface)
	return name + "Plugin"
}

func (g *Generator) clientName(iface *Interface) string {
	name, _ := g.interfaceName(iface)
	return name + "RPCClient"
}

func (g *Generator) serverName(iface *Interface) string {
	name, _ := g.interfaceName(iface)
	return name + "RPCServer"
}

func (g *Generator) paramsStructName(iface *Interface, m *Method) string {
	interfaceName, _ := g.interfaceName(iface)
	return "Z_" + interfaceName + "_" + m.name + "Params"
}

func (g *Generator) resultsStructName(iface *Interface, m *Method) string {
	interfaceName, _ := g.interfaceName(iface)
	return "Z_" + interfaceName + "_" + m.name + "Results"
}
