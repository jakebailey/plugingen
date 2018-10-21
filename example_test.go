package main

import (
	"os"
	"testing"
)

func TestExample(t *testing.T) {
	params := runParams{
		typeList: []string{"Thinger"},
		output:   os.DevNull,
		subPkg:   "exampleplug",
		rpcPanic: true,
		args:     []string{"./example"},
	}

	if err := run(params); err != nil {
		t.Error(err)
	}
}
