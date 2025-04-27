package analyzer

import (
	"flag"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

type readonlyAnalyzer struct {
	readerPkg    string
	dataPkg      string
	targetStruct string
}

func NewAnalyzer(readerPkg, dataPkg, targetStruct string) *analysis.Analyzer {
	a := &readonlyAnalyzer{
		readerPkg:    readerPkg,
		dataPkg:      dataPkg,
		targetStruct: targetStruct,
	}

	return &analysis.Analyzer{
		Name:     "readonly",
		Doc:      "Checks that Reader package doesn't modify target structure (including pointer receivers)",
		Run:      a.run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Flags:    a.newFlagSet(),
	}
}

func (a *readonlyAnalyzer) newFlagSet() flag.FlagSet {
	fs := flag.NewFlagSet("readonly", flag.ExitOnError)
	fs.StringVar(&a.readerPkg, "reader", a.readerPkg, "package name to check")
	fs.StringVar(&a.dataPkg, "data", a.dataPkg, "data package name")
	fs.StringVar(&a.targetStruct, "struct", a.targetStruct, "protected structure name")
	return *fs
}

func (a *readonlyAnalyzer) run(pass *analysis.Pass) (interface{}, error) {
	if pass.Pkg.Name() != a.readerPkg {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.AssignStmt)(nil),
		(*ast.IncDecStmt)(nil),
		(*ast.CallExpr)(nil),
		(*ast.UnaryExpr)(nil), // Для проверки разыменований
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.AssignStmt:
			a.checkAssignment(pass, node)
		case *ast.IncDecStmt:
			a.checkIncDec(pass, node)
		case *ast.CallExpr:
			a.checkCall(pass, node)
		case *ast.UnaryExpr:
			if node.Op == token.AND {
				a.checkAddressOf(pass, node)
			}
		}
	})

	return nil, nil
}

func (a *readonlyAnalyzer) isTargetType(t types.Type) bool {
	// Проверяем как сам тип, так и указатели на него
	switch typ := t.(type) {
	case *types.Pointer:
		return a.isTargetType(typ.Elem())
	case *types.Named:
		return typ.Obj() != nil &&
			typ.Obj().Pkg() != nil &&
			typ.Obj().Pkg().Name() == a.dataPkg &&
			typ.Obj().Name() == a.targetStruct
	}
	return false
}

func (a *readonlyAnalyzer) checkAssignment(pass *analysis.Pass, assign *ast.AssignStmt) {
	for _, lhs := range assign.Lhs {
		// Проверяем присваивание полям структуры
		if sel, ok := lhs.(*ast.SelectorExpr); ok {
			if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
				pass.Reportf(lhs.Pos(), "assignment to field of BigStruct is forbidden")
			}
		}

		// Проверяем присваивание самой структуре
		if t := pass.TypesInfo.TypeOf(lhs); t != nil && a.isTargetType(t) {
			pass.Reportf(lhs.Pos(), "assignment to BigStruct is forbidden")
		}
	}
}

func (a *readonlyAnalyzer) checkIncDec(pass *analysis.Pass, incDec *ast.IncDecStmt) {
	if t := pass.TypesInfo.TypeOf(incDec.X); t != nil && a.isTargetType(t) {
		pass.Reportf(incDec.Pos(), "modification of %s is forbidden", a.targetStruct)
	}
}

func (a *readonlyAnalyzer) checkCall(pass *analysis.Pass, call *ast.CallExpr) {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
			pass.Reportf(call.Pos(), "method call on %s is forbidden", a.targetStruct)
		}
	}
}

func (a *readonlyAnalyzer) checkAddressOf(pass *analysis.Pass, unary *ast.UnaryExpr) {
	if t := pass.TypesInfo.TypeOf(unary.X); t != nil && a.isTargetType(t) {
		pass.Reportf(unary.Pos(), "taking address of %s may lead to modifications", a.targetStruct)
	}
}
