package analyzer

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzerResults(t *testing.T) {
	testdata := analysistest.TestData()
	analyzer := NewAnalyzer("reader", "data", "BigStruct", "writer")

	results := analysistest.Run(t, testdata, analyzer, "invalidreader")

	// Проверяем результаты
	if len(results) == 0 {
		t.Fatal("expected issues, got none")
	}

	issues, ok := results[0].Result.([]Issue)
	if !ok {
		t.Fatalf("expected []Issue, got %T", results[0].Result)
	}

	if len(issues) < 2 {
		t.Errorf("expected at least 2 issues, got %d", len(issues))
	}
}
