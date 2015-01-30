// Copyright (c) 2013 The Go Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd.

// golint lints the Go source files named on its command line.
package main

import (
	"flag"
	"fmt"
	"go/build"
	"go/parser"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/loader"

	"github.com/golang/lint"
)

var minConfidence = flag.Float64("min_confidence", 0.8, "minimum confidence of a problem to print it")

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\tgolint [flags] # runs on package in current directory\n")
	fmt.Fprintf(os.Stderr, "\tgolint [flags] package\n")
	fmt.Fprintf(os.Stderr, "\tgolint [flags] directory\n")
	fmt.Fprintf(os.Stderr, "\tgolint [flags] files... # must be a single package\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	cfg := &loader.Config{
		AllowErrors: true,
		ParserMode:  parser.ParseComments,
	}

	switch flag.NArg() {
	case 0:
		addDir(cfg, ".")
	case 1:
		arg := flag.Arg(0)
		if strings.HasSuffix(arg, "/...") && isDir(arg[:len(arg)-4]) {
			for _, dirname := range allPackagesInFS(arg) {
				addDir(cfg, dirname)
			}
		} else if isDir(arg) {
			addDir(cfg, arg)
		} else if exists(arg) {
			err := cfg.CreateFromFilenames(".", arg)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		} else {
			err := cfg.ImportWithTests(arg)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	default:
		err := cfg.CreateFromFilenames(".", flag.Args()...)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}

	program, err := cfg.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	l := new(lint.Linter)
	var ps []lint.Problem

	for _, pkg := range program.Created {
		pp, err := l.LintFiles(pkg.Files)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			continue
		}

		ps = append(ps, pp...)
	}

	sort.Sort(lint.ByPosition(ps))

	for _, p := range ps {
		if p.Confidence >= *minConfidence {
			fmt.Printf("%v: %s\n", p.Position, p.Text)
		}
	}
}

func isDir(filename string) bool {
	fi, err := os.Stat(filename)
	return err == nil && fi.IsDir()
}

func exists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func addDir(cfg *loader.Config, dirname string) {
	// go/loader does currently not expose ImportDir
	pkg, err := build.ImportDir(dirname, 0)
	if err != nil {
		if _, nogo := err.(*build.NoGoError); nogo {
			// Don't complain if the failure is due to no Go source files.
			return
		}
		fmt.Fprintln(os.Stderr, err)
		return
	}

	var files []string
	files = append(files, pkg.GoFiles...)
	files = append(files, pkg.TestGoFiles...)

	joinDirWithFilenames(dirname, files)

	err = cfg.CreateFromFilenames(".", files...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	if files := pkg.XTestGoFiles; len(files) != 0 {
		joinDirWithFilenames(dirname, files)

		err = cfg.CreateFromFilenames(".", files...)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func joinDirWithFilenames(dir string, files []string) {
	if dir != "." {
		for i, f := range files {
			files[i] = filepath.Join(dir, f)
		}
	}
}
