package analyzer

import (
	"go/types"
	"log"
	"sort"

	"github.com/jakebailey/plugingen/typesext"
	"golang.org/x/tools/go/types/typeutil"
)

type Analyzer struct {
	allowError bool

	done       map[string]*Interface
	interfaces []*Interface

	cache typeutil.MethodSetCache
}

func NewAnalyzer(allowError bool) *Analyzer {
	return &Analyzer{
		allowError: allowError,
		done:       map[string]*Interface{},
	}
}

type Interface struct {
	Typ      types.Type
	Methods  []*Method
	sortName string
}

type Method struct {
	Name     string
	Params   []*Var
	Results  []*Var
	Variadic bool
}

type Var struct {
	Name  string
	Typ   types.Type
	IFace *Interface
}

func (a *Analyzer) AnalyzeAll(ts []types.Type) []*Interface {
	for _, t := range ts {
		a.analyze(t)
	}

	sort.Slice(a.interfaces, func(i, j int) bool {
		return a.interfaces[i].sortName < a.interfaces[j].sortName
	})

	ret := a.interfaces
	a.interfaces = nil

	for k := range a.done {
		delete(a.done, k)
	}

	return ret
}

func (a *Analyzer) analyze(t types.Type) *Interface {
	typeString := t.String()

	if iface, ok := a.done[typeString]; ok {
		return iface
	}

	iface := &Interface{
		Typ:      t,
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
			Name:     methodName,
			Params:   make([]*Var, 0, len(params)),
			Results:  make([]*Var, 0, len(results)),
			Variadic: variadic,
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
				Name: param.Name(),
				Typ:  typ,
			}

			if typesext.IsPluggable(typ) {
				v.IFace = a.analyze(typ)
			} else {
				if typesext.IsEmptyInterface(typ) {
					log.Printf("warning: empty interface parameter in %s.%s may not be compatible", typeString, methodName)
				} else if a.allowError && typesext.IsError(typ) {
					log.Printf("warning: error interface parameter in %s.%s may not be compatible", typeString, methodName)
				} else if typesext.IsPointerLike(typ) {
					log.Printf("warning: pointer-like parameter in %s.%s, writes made in a plugin will not propogate", typeString, methodName)
				}
			}

			method.Params = append(method.Params, v)
		}

		for _, result := range results {
			typ := result.Type()

			v := &Var{
				Name: result.Name(),
				Typ:  typ,
			}

			if typesext.IsPluggable(typ) {
				log.Fatalf("error: non-empty or error interface return in %s.%s is unsupported", typeString, methodName)
			} else {
				if typesext.IsEmptyInterface(typ) {
					log.Printf("warning: empty interface result in %s.%s may not be compatible", typeString, methodName)
				} else if a.allowError && typesext.IsError(typ) {
					log.Printf("warning: error interface result in %s.%s may not be compatible", typeString, methodName)
				}
			}

			method.Results = append(method.Results, v)
		}

		iface.Methods = append(iface.Methods, method)
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
