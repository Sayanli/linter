package main

import (
	"github.com/Sayanli/linter/analyzer"
	"golang.org/x/tools/go/analysis"
)

type analyzerPlugin struct{}

func (*analyzerPlugin) New() []*analysis.Analyzer {
	a := analyzer.NewAnalyzer(
		"default/reader",
		"default/data",
		"TargetStruct",
		"default/writer",
	)
	a.Name = "rwsep"
	return []*analysis.Analyzer{a}
}
