package main

import (
	"go/types"
	"log"

	"golang.org/x/tools/go/types/typeutil"
)

type Analyzer struct {
	done       map[string]*Interface
	interfaces []*Interface

	cache typeutil.MethodSetCache
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		done: map[string]*Interface{},
	}
}

type Interface struct {
	typ     types.Type
	methods []*Method
}

type Method struct {
	name     string
	params   []*Var
	results  []*Var
	variadic bool
}

type Var struct {
	name  string
	typ   types.Type
	iface *Interface
}

func (a *Analyzer) analyze(t types.Type) *Interface {
	typeString := t.String()

	if iface, ok := a.done[typeString]; ok {
		return iface
	}

	iface := &Interface{
		typ: t,
	}

	a.done[typeString] = iface

	for _, sel := range typeutil.IntuitiveMethodSet(t, &a.cache) {
		o := sel.Obj()

		methodName := o.Name()
		sig := o.Type().(*types.Signature)

		params := tupleToSlice(sig.Params())
		results := tupleToSlice(sig.Results())
		variadic := sig.Variadic()

		method := &Method{
			name:     methodName,
			params:   make([]*Var, 0, len(params)),
			results:  make([]*Var, 0, len(results)),
			variadic: variadic,
		}

		if variadic {
			last := params[len(params)-1].Type().Underlying().(*types.Slice).Elem()
			if types.IsInterface(last) {
				log.Fatalf("variadic interface arguments are unsupported: %s.%s", typeString, methodName)
			}
		}

		for _, param := range params {
			typ := param.Type()

			v := &Var{
				name: param.Name(),
				typ:  typ,
			}

			if isPluggable(typ) {
				v.iface = a.analyze(typ)
			}

			method.params = append(method.params, v)
		}

		for _, result := range results {
			typ := result.Type()

			v := &Var{
				name: result.Name(),
				typ:  typ,
			}

			if isPluggable(typ) {
				log.Fatalf("returning an interface is unsupported: %s.%s", typeString, methodName)
			}

			method.results = append(method.results, v)
		}

		iface.methods = append(iface.methods, method)
	}

	a.interfaces = append(a.interfaces, iface)
	return iface
}

func tupleToSlice(tuple *types.Tuple) []*types.Var {
	listLen := tuple.Len()

	if listLen == 0 {
		return nil
	}

	list := make([]*types.Var, listLen)

	for i := 0; i < listLen; i++ {
		list[i] = tuple.At(i)
	}

	return list
}
