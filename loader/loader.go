package loader

import (
	"errors"
	"go/build"
	"go/types"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/loader"
)

var ErrTagsNotApplicable = errors.New("build tags only apply to directories, not when files are specified")

func LoadPackage(buildTags []string, args []string) (pkg *types.Package, dir string, err error) {
	if len(args) == 0 {
		args = []string{"."}
	}

	conf := loader.Config{
		Build:               buildContext(buildTags),
		TypeCheckFuncBodies: func(string) bool { return false },
	}

	if len(args) == 1 && isDirectory(args[0]) {
		dir = args[0]
		conf.Import(dir)
	} else {
		if len(buildTags) != 0 {
			return nil, "", ErrTagsNotApplicable
		}
		dir = filepath.Dir(args[0])
		conf.CreateFromFilenames("", args...)
	}

	lprog, err := conf.Load()
	if err != nil {
		return nil, "", err
	}

	return lprog.InitialPackages()[0].Pkg, dir, nil
}

func buildContext(tags []string) *build.Context {
	ctx := build.Default
	ctx.BuildTags = tags
	return &ctx
}

// isDirectory reports whether the named file is a directory.
func isDirectory(name string) bool {
	info, err := os.Stat(name)
	if err != nil {
		log.Fatal(err)
	}
	return info.IsDir()
}
