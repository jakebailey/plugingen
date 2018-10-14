package main

import "go/types"

func isEmptyInterface(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	return ok && iface.Empty()
}

func isError(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	return ok && iface.String() == "interface{Error() string}"
}

func isPluggable(t types.Type) bool {
	return types.IsInterface(t) && !isEmptyInterface(t) && !isError(t)
}
