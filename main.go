package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/types"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/jakebailey/plugingen/analyzer"
	"github.com/jakebailey/plugingen/generator"
	"github.com/jakebailey/plugingen/loader"
)

var (
	typeNames  = flag.String("type", "", "comma-separated list of type names; must be set")
	output     = flag.String("output", "", "output file name (or - for stdout); default <srcdir>/plugingen.go")
	buildTags  = flag.String("tags", "", "comma-separated list of build tags to apply")
	allowError = flag.Bool("allowerror", false, "don't wrap errors with plugin.BasicError")
	subPkg     = flag.String("subpkg", "", "subpackage name for generated code; if specified, output will be written to <srcdir>/<subpkg>/<output>")
	rpcPanic   = flag.Bool("panicrpc", false, "panic on RPC call errors")
)

// Usage is a replacement usage function for the flags package.
func Usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\tplugingen [flags] -type T [directory]\n")
	fmt.Fprintf(os.Stderr, "\tplugingen [flags] -type T files... # Must be a single package\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("plugingen: ")
	flag.Usage = Usage
	flag.Parse()

	if len(*typeNames) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	params := runParams{
		typeList:   strings.Split(*typeNames, ","),
		output:     *output,
		buildTags:  strings.Split(*buildTags, ","),
		allowError: *allowError,
		subPkg:     *subPkg,
		rpcPanic:   *rpcPanic,
		args:       flag.Args(),
	}

	if err := run(params); err != nil {
		log.Fatal(err)
	}
}

type runParams struct {
	typeList   []string
	output     string
	buildTags  []string
	allowError bool
	subPkg     string
	rpcPanic   bool
	args       []string
}

func run(params runParams) error {
	pkg, dir, err := loader.LoadPackage(params.buildTags, params.args)
	if err != nil {
		return err
	}

	a := analyzer.NewAnalyzer(params.allowError)

	typeList := make([]types.Type, len(params.typeList))
	for i, name := range params.typeList {
		obj := pkg.Scope().Lookup(name)
		if obj == nil {
			return fmt.Errorf("%s.%s not found", pkg.Path(), name)
		}
		typeList[i] = obj.Type()
	}

	ifaces := a.AnalyzeAll(typeList)

	pkgPath := pkg.Path()
	if params.subPkg != "" {
		pkgPath += "/" + params.subPkg
		dir = filepath.Join(dir, params.subPkg)
	}

	file := jen.NewFilePath(pkgPath)
	file.PackageComment(fmt.Sprintf("// Code generated by \"plugingen %s\"; DO NOT EDIT.\n", strings.Join(os.Args[1:], " ")))

	g := generator.NewGenerator(params.allowError, params.rpcPanic, file)
	g.Generate(ifaces)

	var buf bytes.Buffer
	if err := file.Render(&buf); err != nil {
		return err
	}

	outputName := params.output

	if outputName == "-" {
		_, err = buf.WriteTo(os.Stdout)
	} else {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}

		if outputName == "" {
			outputName = filepath.Join(dir, "plugingen.go")
		}

		err = ioutil.WriteFile(outputName, buf.Bytes(), 0644)
	}

	if err != nil {
		return err
	}

	return nil
}
