package main

import (
	"fmt"
	"go/types"
)

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

var paramNameMap = map[int]string{}

func paramName(i int) string {
	if name, ok := paramNameMap[i]; ok {
		return name
	}

	name := fmt.Sprintf("p%d", i)
	paramNameMap[i] = name
	return name
}

var paramNameExMap = map[int]string{}

func paramNameEx(i int) string {
	if name, ok := paramNameExMap[i]; ok {
		return name
	}

	name := fmt.Sprintf("P%d", i)
	paramNameExMap[i] = name
	return name
}

var resultNameMap = map[int]string{}

func resultName(i int) string {
	if name, ok := resultNameMap[i]; ok {
		return name
	}

	name := fmt.Sprintf("r%d", i)
	resultNameMap[i] = name
	return name
}

var resultNameExMap = map[int]string{}

func resultNameEx(i int) string {
	if name, ok := resultNameExMap[i]; ok {
		return name
	}

	name := fmt.Sprintf("R%d", i)
	resultNameExMap[i] = name
	return name
}
