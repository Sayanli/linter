package analyzer

import (
	"fmt"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestReadonlyAnalyzer(t *testing.T) {
	testCases := []struct {
		name        string
		pkg         string
		expectError bool
	}{
		{
			name:        "valid reader package",
			pkg:         "validreader",
			expectError: false,
		},
		{
			name:        "invalid assignments",
			pkg:         "invalidreader",
			expectError: true,
		},
	}

	testdata := analysistest.TestData()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			analyzer := NewAnalyzer(
				"reader",    // Проверяем только пакет 'reader'
				"data",      // Структуры из пакета 'data'
				"BigStruct", // Защищаемая структура
			)

			results := analysistest.Run(t, testdata, analyzer, tc.pkg)
			if tc.expectError {
				if len(results) == 0 || len(results[0].Diagnostics) == 0 {
					t.Errorf("Ожидались ошибки, но их не было")
				}
			} else {
				fmt.Println("err")
				for _, r := range results {
					if len(r.Diagnostics) > 0 {
						t.Errorf("Неожиданные ошибки: %v", r.Diagnostics)
					}
				}
			}
		})
	}
}
