package analyzer

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strings"

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

type FunctionInfo struct {
	FuncDecl   *ast.FuncDecl
	Parameters map[int]struct{}
	Modifies   bool
	Calls      []string
}

type RWsepAnalyzer struct {
	readerPkg    string
	writerPkg    string
	dataPkg      string
	targetStruct string
	issues       []Issue
	aliases      map[string]*Alias
	functions    map[string]*FunctionInfo
	currentFunc  string
}

func NewAnalyzer(readerPkg, dataPkg, targetStruct, writerPkg string) *analysis.Analyzer {
	a := &RWsepAnalyzer{
		readerPkg:    readerPkg,
		writerPkg:    writerPkg,
		dataPkg:      dataPkg,
		targetStruct: targetStruct,
		aliases:      make(map[string]*Alias),
		functions:    make(map[string]*FunctionInfo),
	}

	return &analysis.Analyzer{
		Name:       "rwsep",
		Doc:        "Checks for separation of read and write packages",
		Run:        a.run,
		Requires:   []*analysis.Analyzer{inspect.Analyzer},
		Flags:      a.newFlagSet(),
		ResultType: reflect.TypeOf([]Issue{}),
	}
}

func (a *RWsepAnalyzer) newFlagSet() flag.FlagSet {
	fs := flag.NewFlagSet("rwsep", flag.ExitOnError)
	fs.StringVar(&a.readerPkg, "reader", a.readerPkg, "package name to check read only")
	fs.StringVar(&a.writerPkg, "writer", a.writerPkg, "package name to check write only")
	fs.StringVar(&a.dataPkg, "data", a.dataPkg, "data package name")
	fs.StringVar(&a.targetStruct, "struct", a.targetStruct, "protected structure name")
	return *fs
}

func (a *RWsepAnalyzer) getFunctionName(funcDecl *ast.FuncDecl) string {
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		recvType := funcDecl.Recv.List[0].Type
		var recvTypeName string

		switch t := recvType.(type) {
		case *ast.Ident:
			recvTypeName = t.Name
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				recvTypeName = "*" + ident.Name
			} else {
				recvTypeName = "unknown"
			}
		default:
			recvTypeName = "unknown"
		}

		return fmt.Sprintf("%s.%s", recvTypeName, funcDecl.Name.Name)
	}

	return funcDecl.Name.Name
}

func (a *RWsepAnalyzer) run(pass *analysis.Pass) (interface{}, error) {
	if pass.Pkg.Name() != a.readerPkg {
		return []Issue{}, nil
	}

	a.issues = make([]Issue, 0)
	a.aliases = make(map[string]*Alias)
	a.functions = make(map[string]*FunctionInfo)

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.AssignStmt)(nil),
		(*ast.IncDecStmt)(nil),
		(*ast.CallExpr)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.FuncDecl:
			a.registerFunction(pass, node)
		case *ast.AssignStmt:
			a.checkAssignment(pass, node)
		case *ast.IncDecStmt:
			a.checkIncDec(pass, node)
		case *ast.CallExpr:
			a.checkCallExpr(pass, node)
		}
	})

	a.analyzeFunctionCalls(pass)

	for _, issue := range a.issues {
		pass.Reportf(issue.Pos, issue.Message)
	}

	return a.issues, nil
}

func (a *RWsepAnalyzer) registerFunction(pass *analysis.Pass, funcDecl *ast.FuncDecl) {
	funcName := a.getFunctionName(funcDecl)

	a.functions[funcName] = &FunctionInfo{
		FuncDecl:   funcDecl,
		Parameters: make(map[int]struct{}),
		Calls:      make([]string, 0),
	}
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
	for _, issue := range a.issues {
		if issue.Pos == pos && issue.Message == message {
			return
		}
	}
	a.issues = append(a.issues, Issue{Pos: pos, Message: message})
}

func (a *RWsepAnalyzer) checkAssignment(pass *analysis.Pass, assign *ast.AssignStmt) {
	for i, lhs := range assign.Lhs {
		switch expr := lhs.(type) {
		case *ast.SelectorExpr:
			if t := pass.TypesInfo.TypeOf(expr.X); t != nil && a.isTargetType(t) {
				a.addIssue(expr.Pos(), "direct assignment to protected structure field is forbidden")
				if a.currentFunc != "" {
					if info, ok := a.functions[a.currentFunc]; ok {
						info.Modifies = true
					}
				}
			}
		case *ast.StarExpr:
			if ident, ok := expr.X.(*ast.Ident); ok {
				if sel := a.resolveAlias(ident.Name); sel != nil {
					if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
						a.addIssue(expr.Pos(), "modification through pointer to protected structure field is forbidden")

						if a.currentFunc != "" {
							if info, ok := a.functions[a.currentFunc]; ok {
								info.Modifies = true
							}
						}
					}
				}
			}
		case *ast.IndexExpr:
			// r.d.Values[i] = ...
			if sel, ok := expr.X.(*ast.SelectorExpr); ok {
				if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
					a.addIssue(expr.Pos(), "modification of protected structure field is forbidden")

					if a.currentFunc != "" {
						if info, ok := a.functions[a.currentFunc]; ok {
							info.Modifies = true
						}
					}
				}
			}
			// alias[i] = ...
			if ident, ok := expr.X.(*ast.Ident); ok {
				if sel := a.resolveAlias(ident.Name); sel != nil {
					if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
						a.addIssue(expr.Pos(), "modification through pointer to protected structure field is forbidden")

						if a.currentFunc != "" {
							if info, ok := a.functions[a.currentFunc]; ok {
								info.Modifies = true
							}
						}
					}
				}
			}
			// (*ptr)[i] = ...
			if paren, ok := expr.X.(*ast.ParenExpr); ok {
				if star, ok := paren.X.(*ast.StarExpr); ok {
					if ident, ok := star.X.(*ast.Ident); ok {
						if sel := a.resolveAlias(ident.Name); sel != nil {
							if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
								a.addIssue(expr.Pos(), "modification through pointer to protected structure field is forbidden")

								if a.currentFunc != "" {
									if info, ok := a.functions[a.currentFunc]; ok {
										info.Modifies = true
									}
								}
							}
						}
					}
				}
			}
		}

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
			a.addIssue(incDec.Pos(), "increment/decrement of protected structure field is forbidden")

			if a.currentFunc != "" {
				if info, ok := a.functions[a.currentFunc]; ok {
					info.Modifies = true
				}
			}
		}
	case *ast.StarExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			if sel := a.resolveAlias(ident.Name); sel != nil {
				if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
					a.addIssue(incDec.Pos(), "increment/decrement through pointer to protected structure field is forbidden")

					if a.currentFunc != "" {
						if info, ok := a.functions[a.currentFunc]; ok {
							info.Modifies = true
						}
					}
				}
			}
		}
	}
}

func (a *RWsepAnalyzer) getCallName(pass *analysis.Pass, call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if obj, ok := pass.TypesInfo.Uses[fun]; ok {
			if pkg := obj.Pkg(); pkg != nil {
				return pkg.Name() + "." + fun.Name
			}
		}
		return fun.Name

	case *ast.SelectorExpr:
		if pkgIdent, ok := fun.X.(*ast.Ident); ok {
			if obj, ok := pass.TypesInfo.Uses[pkgIdent]; ok {
				if pkgName, ok := obj.(*types.PkgName); ok {
					return pkgName.Imported().Name() + "." + fun.Sel.Name
				}
			}
		}

		if t := pass.TypesInfo.TypeOf(fun.X); t != nil {
			typeStr := t.String()
			lastDot := strings.LastIndex(typeStr, ".")
			if lastDot != -1 {
				typeName := typeStr[lastDot+1:]
				return typeName + "." + fun.Sel.Name
			}
			return typeStr + "." + fun.Sel.Name
		}
		return fun.Sel.Name
	}

	return "unknown"
}

func (a *RWsepAnalyzer) checkCallExpr(pass *analysis.Pass, call *ast.CallExpr) {
	callName := a.getCallName(pass, call)

	if a.currentFunc != "" {
		if info, ok := a.functions[a.currentFunc]; ok {
			info.Calls = append(info.Calls, callName)
		}
	}

	for i, arg := range call.Args {
		switch expr := arg.(type) {
		case *ast.UnaryExpr:
			if expr.Op == token.AND {
				if sel, ok := expr.X.(*ast.SelectorExpr); ok {
					if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
						if info, ok := a.functions[callName]; ok {
							info.Parameters[i] = struct{}{}
						}

						a.addIssue(call.Pos(), fmt.Sprintf("passing pointer to protected structure field to function %s", callName))
					}
				}
			}
		case *ast.Ident:
			if sel := a.resolveAlias(expr.Name); sel != nil {
				if t := pass.TypesInfo.TypeOf(sel.X); t != nil && a.isTargetType(t) {
					if info, ok := a.functions[callName]; ok {
						info.Parameters[i] = struct{}{}
					}

					a.addIssue(call.Pos(), fmt.Sprintf("passing pointer to protected structure field to function %s", callName))
				}
			}
		}
	}
}

func (a *RWsepAnalyzer) analyzeFunctionCalls(pass *analysis.Pass) {
	visited := make(map[string]bool)

	for funcName := range a.functions {
		a.checkFunctionRecursive(pass, funcName, visited)
	}
}

func (a *RWsepAnalyzer) checkFunctionRecursive(pass *analysis.Pass, funcName string, visited map[string]bool) bool {
	info, exists := a.functions[funcName]
	if !exists {
		return false
	}

	if info.Modifies {
		return true
	}

	if visited[funcName] {
		return false
	}
	visited[funcName] = true

	modifies := false

	for _, calleeName := range info.Calls {
		if a.checkFunctionRecursive(pass, calleeName, visited) {
			modifies = true

			var callPos token.Pos
			if info.FuncDecl != nil && info.FuncDecl.Body != nil {
				ast.Inspect(info.FuncDecl.Body, func(n ast.Node) bool {
					if call, ok := n.(*ast.CallExpr); ok {
						if a.getCallName(pass, call) == calleeName {
							callPos = call.Pos()
							return false
						}
					}
					return true
				})
			}

			if callPos != token.NoPos {
				a.addIssue(callPos, fmt.Sprintf("calling function %s that modifies protected structure field through pointer parameter", calleeName))
			}
		}
	}

	if modifies {
		info.Modifies = true
	}

	return info.Modifies
}
