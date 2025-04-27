package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
)

// Analyzer представляет анализатор кода
type analyzer struct {
	fset *token.FileSet
}

func (a *analyzer) NewFlagSet() flag.FlagSet {
	fs := flag.NewFlagSet("", flag.PanicOnError)
	fs.String("r", "", "reader package")
	fs.String("w", "", "writer package")
	return *fs
}

// NewAnalyzer создает новый анализатор
func NewAnalyzer() *analyzer {
	return &analyzer{
		fset: token.NewFileSet(),
	}
}

// AnalyzePackage анализирует все файлы указанного пакета
func (a *analyzer) AnalyzePackage(pkgPath string) {
	// Находим все .go файлы в директории пакета
	files, err := filepath.Glob(filepath.Join(pkgPath, "*.go"))
	if err != nil {
		log.Fatalf("Error finding Go files: %v", err)
	}

	if len(files) == 0 {
		log.Fatalf("No Go files found in package: %s", pkgPath)
	}

	fmt.Printf("Analyzing package at: %s\n", pkgPath)
	fmt.Printf("Found %d Go files\n\n", len(files))

	// Анализируем каждый файл
	for _, file := range files {
		a.analyzeFile(file)
	}
}

// analyzeFile анализирует отдельный файл
func (a *analyzer) analyzeFile(filename string) {
	fmt.Printf("=== Analyzing file: %s ===\n", filename)

	// Парсим файл
	file, err := parser.ParseFile(a.fset, filename, nil, parser.ParseComments)
	if err != nil {
		log.Printf("Error parsing file %s: %v", filename, err)
		return
	}

	// Заглушка для проверки
	a.dummyCheck(file)

	// Выводим AST (можно закомментировать)
	ast.Print(a.fset, file)
	fmt.Println()
}

// dummyCheck - заглушка для проверки
func (a *analyzer) dummyCheck(file *ast.File) {
	fmt.Printf("[Dummy Check] Package: %s\n", file.Name)

	// Простая проверка: считаем количество функций
	funcCount := 0
	ast.Inspect(file, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncDecl); ok {
			funcCount++
		}
		return true
	})

	fmt.Printf("[Dummy Check] Found %d functions\n", funcCount)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: goastlinter <package-path>")
		fmt.Println("Example: goastlinter ./myproject/mypackage")
		os.Exit(1)
	}

	pkgPath := os.Args[1]

	// Создаем анализатор
	analyzer := NewAnalyzer()

	// Анализируем пакет
	analyzer.AnalyzePackage(pkgPath)
}
