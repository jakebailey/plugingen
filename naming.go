package main

import "fmt"

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
