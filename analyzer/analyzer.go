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

type Alias struct {
	Name string
	Expr *ast.SelectorExpr
	Prev *Alias
}

type RWsepAnalyzer struct {
	readerPkg    string
	writerPkg    string
	dataPkg      string
	targetStruct string
	issues       []Issue
	aliases      map[string]*Alias
}

func NewAnalyzer(readerPkg, dataPkg, targetStruct, writerPkg string) *analysis.Analyzer {
	a := &RWsepAnalyzer{
		readerPkg:    readerPkg,
		writerPkg:    writerPkg,
		dataPkg:      dataPkg,
		targetStruct: targetStruct,
		aliases:      make(map[string]*Alias),
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

func (a *RWsepAnalyzer) newFlagSet() flag.FlagSet {
	fs := flag.NewFlagSet("readonly", flag.ExitOnError)
	fs.StringVar(&a.readerPkg, "reader", a.readerPkg, "package name to check read only")
	fs.StringVar(&a.writerPkg, "writer", a.writerPkg, "package name to check write only")
	fs.StringVar(&a.dataPkg, "data", a.dataPkg, "data package name")
	fs.StringVar(&a.targetStruct, "struct", a.targetStruct, "protected structure name")
	return *fs
}

func (a *RWsepAnalyzer) run(pass *analysis.Pass) (interface{}, error) {
	if pass.Pkg.Name() != a.readerPkg {
		return []Issue{}, nil
	}

	a.issues = make([]Issue, 0)
	a.aliases = make(map[string]*Alias)

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.AssignStmt)(nil),
		(*ast.IncDecStmt)(nil),
		(*ast.CallExpr)(nil),
		(*ast.UnaryExpr)(nil),
		(*ast.RangeStmt)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.AssignStmt:
			a.checkAssignment(pass, node)
		case *ast.IncDecStmt:
			a.checkIncDec(pass, node)
		case *ast.CallExpr:
			a.checkCall(pass, node)
		case *ast.RangeStmt:
			a.checkRange(pass, node)
		}
	})

	for _, issue := range a.issues {
		pass.Reportf(issue.Pos, issue.Message)
	}

	return a.issues, nil
}

func (a *RWsepAnalyzer) resolveAlias(name string) *ast.SelectorExpr {
	seen := make(map[string]bool)
	curr := a.aliases[name]

	for curr != nil {
		if seen[curr.Name] {
			break
		}
		seen[curr.Name] = true
		if curr.Expr != nil {
			return curr.Expr
		}
		curr = curr.Prev
	}
	return nil
}

func (a *RWsepAnalyzer) isTargetType(t types.Type) bool {
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

func (a *RWsepAnalyzer) addIssue(pos token.Pos, message string) {
	a.issues = append(a.issues, Issue{Pos: pos, Message: message})
}

func (a *RWsepAnalyzer) checkAssignment(pass *analysis.Pass, assign *ast.AssignStmt) {
	for i, lhs := range assign.Lhs {
		switch expr := lhs.(type) {
		case *ast.SelectorExpr:
			if t := pass.TypesInfo.TypeOf(expr.X); t != nil && a.isTargetType(t) {
				a.addIssue(expr.Pos(), "direct assignment to field of "+a.targetStruct+" is forbidden")
			}
		case *ast.StarExpr:
			if ident, ok := expr.X.(*ast.Ident); ok {
				if sel := a.resolveAlias(ident.Name); sel != nil {
					if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
						a.addIssue(expr.Pos(), "modification through pointer to "+a.targetStruct+" field is forbidden")
					}
				}
			}
		}

		// отслеживание алиасов
		if ident, ok := lhs.(*ast.Ident); ok && i < len(assign.Rhs) {
			switch rhs := assign.Rhs[i].(type) {
			case *ast.UnaryExpr:
				if rhs.Op == token.AND {
					if sel, ok := rhs.X.(*ast.SelectorExpr); ok {
						if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
							a.aliases[ident.Name] = &Alias{Name: ident.Name, Expr: sel, Prev: nil}
						}
					}
				}
			case *ast.Ident:
				if prev, found := a.aliases[rhs.Name]; found {
					a.aliases[ident.Name] = &Alias{Name: ident.Name, Prev: prev}
				}
			}
		}
	}
}

func (a *RWsepAnalyzer) checkIncDec(pass *analysis.Pass, incDec *ast.IncDecStmt) {
	switch expr := incDec.X.(type) {
	case *ast.SelectorExpr:
		if t := pass.TypesInfo.TypeOf(expr.X); t != nil && a.isTargetType(t) {
			a.addIssue(incDec.Pos(), "increment/decrement of "+a.targetStruct+" field is forbidden")
		}
	case *ast.StarExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			if sel := a.resolveAlias(ident.Name); sel != nil {
				if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
					a.addIssue(incDec.Pos(), "increment/decrement through pointer to "+a.targetStruct+" field is forbidden")
				}
			}
		}
	}
}

func (a *RWsepAnalyzer) checkCall(pass *analysis.Pass, call *ast.CallExpr) {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
			a.addIssue(call.Pos(), "method call on "+a.targetStruct+" is forbidden")
		}
	}
}

func (a *RWsepAnalyzer) checkRange(pass *analysis.Pass, rng *ast.RangeStmt) {
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
