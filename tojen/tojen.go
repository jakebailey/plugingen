// Package tojen implements conversion from go/types to jen statements.
package tojen

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/dave/jennifer/jen"
)

func Type(t types.Type) *jen.Statement {
	return typeToJen(t, nil)
}

func typeToJen(t types.Type, visited []types.Type) *jen.Statement {
	for _, v := range visited {
		if v == t {
			return jen.Id(fmt.Sprintf("○%T", t))
		}
	}
	visited = append(visited, t)

	switch t := t.(type) {
	case nil:
		return jen.Id("<nil>")

	case *types.Basic:
		if t.Kind() == types.UnsafePointer {
			return jen.Qual("unsafe", t.Name())
		}
		return jen.Id(t.Name())

	case *types.Array:
		return jen.Index(jen.Lit(t.Len())).Add(typeToJen(t.Elem(), visited))

	case *types.Slice:
		return jen.Index().Add(typeToJen(t.Elem(), visited))

	case *types.Struct:
		numFields := t.NumFields()
		return jen.StructFunc(func(g *jen.Group) {
			for i := 0; i < numFields; i++ {
				field := t.Field(i)
				code := g.Null()

				if !field.Embedded() {
					code = code.Id(field.Name())
				}

				code.Add(typeToJen(field.Type(), visited))

				// TODO: tag rendering (unsupported by jen without reparsing)
				// code.Tag(t.Tag(i))
			}
		})

	case *types.Pointer:
		return jen.Op("*").Add(typeToJen(t.Elem(), visited))

	case *types.Tuple:
		return tupleToJen(t, false, visited)

	case *types.Signature:
		return jen.Func().Add(signatureToJen(t, visited))

	case *types.Interface:
		return jen.InterfaceFunc(func(g *jen.Group) {
			numMethods := t.NumExplicitMethods()

			for i := 0; i < numMethods; i++ {
				m := t.ExplicitMethod(i)
				g.Id(m.Name()).Add(signatureToJen(m.Type().(*types.Signature), visited))
			}

			numEmbeddeds := t.NumEmbeddeds()

			for i := 0; i < numEmbeddeds; i++ {
				typ := t.EmbeddedType(i)
				g.Add(typeToJen(typ, visited))
			}
		})

	case *types.Map:
		return jen.Map(typeToJen(t.Key(), visited)).Add(typeToJen(t.Elem(), visited))

	case *types.Chan:
		var code *jen.Statement
		parens := false

		switch t.Dir() {
		case types.SendRecv:
			code = jen.Chan()

			if c, _ := t.Elem().(*types.Chan); c != nil && c.Dir() == types.RecvOnly {
				parens = true
			}

		case types.SendOnly:
			code = jen.Chan().Op("<-")

		case types.RecvOnly:
			code = jen.Op("<-").Chan()

		default:
			panic("unreachable")
		}

		j := typeToJen(t.Elem(), visited)

		if parens {
			return code.Parens(j)
		}

		return code.Add(j)

	case *types.Named:
		if obj := t.Obj(); obj != nil {
			if pkg := obj.Pkg(); pkg != nil {
				path := pkg.Path()

				// TODO: Fix this hack for vendored dependencies.
				if i := strings.LastIndex(path, "/vendor/"); i != -1 {
					i += len("/vendor/")
					path = path[i:]
				}

				return jen.Qual(path, obj.Name())
			}
			return jen.Id(obj.Name())
		}
	}

	panic("unreachable")
}

func Tuple(t *types.Tuple, variadic bool) *jen.Statement {
	return tupleToJen(t, variadic, nil)
}

func tupleToJen(t *types.Tuple, variadic bool, visited []types.Type) *jen.Statement {
	return jen.ParamsFunc(func(g *jen.Group) {
		tupleLen := t.Len()

		if variadic {
			tupleLen--
		}

		for i := 0; i < tupleLen; i++ {
			v := t.At(i)
			j := typeToJen(v.Type(), visited)
			name := v.Name()

			if name == "" {
				g.Add(j)
			} else {
				g.Id(name).Add(j)
			}
		}

		if variadic {
			v := t.At(tupleLen)
			code := g.Null()

			if name := v.Name(); name != "" {
				code = code.Id(name)
			}

			typ := v.Type()

			if s, ok := typ.(*types.Slice); ok {
				code = code.Op("...")
				typ = s.Elem()
			} else {
				if b, ok := typ.Underlying().(*types.Basic); !ok || b.Kind() != types.String {
					panic("internal error: string type expected")
				}

				j := typeToJen(typ, visited)
				code.Add(j).Op("...")
				return
			}

			j := typeToJen(typ, visited)
			code.Add(j)
		}
	})
}

func Signature(t *types.Signature) *jen.Statement {
	return signatureToJen(t, nil)
}

func signatureToJen(t *types.Signature, visited []types.Type) *jen.Statement {
	code := tupleToJen(t.Params(), t.Variadic(), visited)

	n := t.Results().Len()

	if n == 0 {
		return code
	}

	if n == 1 && t.Results().At(0).Name() == "" {
		return code.Add(typeToJen(t.Results().At(0).Type(), visited))
	}

	return code.Add(tupleToJen(t.Results(), false, visited))
}
