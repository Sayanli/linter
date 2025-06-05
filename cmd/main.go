package main

import (
	"flag"

	"github.com/Sayanli/linter/analyzer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

var (
	fs = flag.NewFlagSet("readonly", flag.ExitOnError)
	r  = fs.String("reader", "reader", "path to the reader file")
	w  = fs.String("writer", "writer", "path to the writer file")
	d  = fs.String("data", "data", "data package name")
	s  = fs.String("structure", "structure", "protected structure name")
)

func main() {
	singlechecker.Main(analyzer.NewAnalyzer(*r, *w, *d, *s))
}
