package typesext

import "go/types"

func IsEmptyInterface(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	return ok && iface.Empty()
}

func IsError(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	return ok && iface.String() == "interface{Error() string}"
}

func IsPluggable(t types.Type) bool {
	return types.IsInterface(t) && !IsEmptyInterface(t) && !IsError(t)
}

func IsPointerLike(t types.Type) bool {
	t = t.Underlying()
	switch t.(type) {
	case *types.Pointer, *types.Map, *types.Chan:
		return true
	}

	return false
}
