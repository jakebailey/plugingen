package main

import (
	"go/types"
	"log"
	"sort"

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
	typ      types.Type
	methods  []*Method
	sortName string
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

func (a *Analyzer) analyzeAll(ts []types.Type) {
	for _, t := range ts {
		a.analyze(t)
	}

	sort.Slice(a.interfaces, func(i, j int) bool {
		return a.interfaces[i].sortName < a.interfaces[j].sortName
	})
}

func (a *Analyzer) analyze(t types.Type) *Interface {
	typeString := t.String()

	if iface, ok := a.done[typeString]; ok {
		return iface
	}

	iface := &Interface{
		typ:      t,
		sortName: typeString,
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
				log.Fatalf("error: variadic interface arguments in %s.%s are unsupported", typeString, methodName)
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
			} else {
				if isEmptyInterface(typ) {
					log.Printf("warning: empty interface parameter in %s.%s may not be compatible", typeString, methodName)
				} else if *allowerror && isError(typ) {
					log.Printf("warning: error interface parameter in %s.%s may not be compatible", typeString, methodName)
				}
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
				log.Fatalf("error: non-empty or error interface return in %s.%s is unsupported", typeString, methodName)
			} else {
				if isEmptyInterface(typ) {
					log.Printf("warning: empty interface result in %s.%s may not be compatible", typeString, methodName)
				} else if *allowerror && isError(typ) {
					log.Printf("warning: error interface result in %s.%s may not be compatible", typeString, methodName)
				}
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
