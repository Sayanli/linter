package analyzer

import (
	"flag"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

type Issue struct {
	Pos     token.Pos
	Message string
}

type readonlyAnalyzer struct {
	readerPkg    string
	dataPkg      string
	targetStruct string
	issues       []Issue
}

func NewAnalyzer(readerPkg, dataPkg, targetStruct string) *analysis.Analyzer {
	a := &readonlyAnalyzer{
		readerPkg:    readerPkg,
		dataPkg:      dataPkg,
		targetStruct: targetStruct,
	}

	return &analysis.Analyzer{
		Name:       "todo",
		Doc:        "todo",
		Run:        a.run,
		Requires:   []*analysis.Analyzer{inspect.Analyzer},
		Flags:      a.newFlagSet(),
		ResultType: reflect.TypeOf([]Issue{}),
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
		return []Issue{}, nil
	}

	a.issues = make([]Issue, 0)

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.AssignStmt)(nil), // =, +=, -=, ...
		(*ast.IncDecStmt)(nil), // ++, --
		(*ast.CallExpr)(nil),
		(*ast.UnaryExpr)(nil), // &
		(*ast.RangeStmt)(nil), // range
		(*ast.FuncDecl)(nil),
		//TODO добавить проверку указателей
		//(*ast.Ident)(nil),
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
		case *ast.RangeStmt:
			a.checkRange(pass, node)
		case *ast.FuncDecl:
			a.checkReceiver(pass, node)
			//case *ast.Ident:
			//	a.checkIdent(pass, node)
		}
	})

	for _, issue := range a.issues {
		pass.Reportf(issue.Pos, issue.Message)
	}

	return a.issues, nil
}

func (a *readonlyAnalyzer) isTargetType(t types.Type) bool {
	switch typ := t.(type) {
	case *types.Pointer:
		return a.isTargetType(typ.Elem())
	case *types.Named:
		return typ.Obj() != nil &&
			typ.Obj().Pkg() != nil &&
			typ.Obj().Pkg().Name() == a.dataPkg &&
			typ.Obj().Name() == a.targetStruct
	case *types.Struct:
		for i := 0; i < typ.NumFields(); i++ {
			if a.isTargetType(typ.Field(i).Type()) {
				return true
			}
		}
	}
	return false
}

func (a *readonlyAnalyzer) addIssue(pos token.Pos, message string) {
	a.issues = append(a.issues, Issue{Pos: pos, Message: message})
}

func (a *readonlyAnalyzer) checkAssignment(pass *analysis.Pass, assign *ast.AssignStmt) {
	for _, lhs := range assign.Lhs {
		if sel, ok := lhs.(*ast.SelectorExpr); ok {
			if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
				a.addIssue(lhs.Pos(), "assignment to field of "+a.targetStruct+" is forbidden")
			}
		}
		if t := pass.TypesInfo.TypeOf(lhs); t != nil && a.isTargetType(t) {
			a.addIssue(lhs.Pos(), "assignment to "+a.targetStruct+" is forbidden")
		}
	}
}

func (a *readonlyAnalyzer) checkIncDec(pass *analysis.Pass, incDec *ast.IncDecStmt) {
	if sel, ok := incDec.X.(*ast.SelectorExpr); ok {
		if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
			a.addIssue(incDec.Pos(), "IncDec modification of "+a.targetStruct+" is forbidden")
		}
	}
	if t := pass.TypesInfo.TypeOf(incDec.X); t != nil && a.isTargetType(t) {
		a.addIssue(incDec.Pos(), "IncDec modification of "+a.targetStruct+" is forbidden")
	}
}

func (a *readonlyAnalyzer) checkCall(pass *analysis.Pass, call *ast.CallExpr) {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
			a.addIssue(call.Pos(), "method call on "+a.targetStruct+" is forbidden")
		}
	}
}

func (a *readonlyAnalyzer) checkAddressOf(pass *analysis.Pass, unary *ast.UnaryExpr) {
	if t := pass.TypesInfo.TypeOf(unary.X); t != nil && a.isTargetType(t) {
		a.addIssue(unary.Pos(), "taking address of "+a.targetStruct+" may lead to modifications")
	}
}

func (a *readonlyAnalyzer) checkRange(pass *analysis.Pass, rng *ast.RangeStmt) {
	if rng.Key != nil {
		if t := pass.TypesInfo.TypeOf(rng.Key); t != nil && a.isTargetType(t) {
			a.addIssue(rng.Key.Pos(), "range key variable of type "+a.targetStruct+" is forbidden")
		}
	}
	if rng.Value != nil {
		if t := pass.TypesInfo.TypeOf(rng.Value); t != nil && a.isTargetType(t) {
			a.addIssue(rng.Value.Pos(), "range value variable of type "+a.targetStruct+" is forbidden")
		}
	}
}

func (a *readonlyAnalyzer) checkReceiver(pass *analysis.Pass, fn *ast.FuncDecl) {
	if fn.Recv != nil {
		for _, field := range fn.Recv.List {
			for _, name := range field.Names {
				if t := pass.TypesInfo.TypeOf(name); t != nil && a.isTargetType(t) {
					a.addIssue(fn.Pos(), "method with "+a.targetStruct+" receiver is forbidden")
				}
			}
		}
	}
}
