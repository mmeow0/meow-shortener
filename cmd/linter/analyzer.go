package main

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer проверяет использование panic, log.Fatal и os.Exit вне функции main пакета main.
var Analyzer = &analysis.Analyzer{
	Name: "noexit",
	Doc:  "сообщает об использовании panic и вызовах log.Fatal/os.Exit вне функции main пакета main",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		filename := pass.Fset.File(file.Pos()).Name()
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}

		mainBody := findMainBody(pass, file)

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			checkPanic(pass, call)
			checkForbiddenCall(pass, call, mainBody)

			return true
		})
	}

	return nil, nil
}

// findMainBody возвращает тело функции main, если файл принадлежит пакету main.
func findMainBody(pass *analysis.Pass, file *ast.File) *ast.BlockStmt {
	if pass.Pkg.Name() != "main" {
		return nil
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "main" && fn.Body != nil {
			return fn.Body
		}
	}
	return nil
}

// checkPanic сообщает об использовании встроенной функции panic.
func checkPanic(pass *analysis.Pass, call *ast.CallExpr) {
	ident, ok := call.Fun.(*ast.Ident)
	if ok && ident.Name == "panic" {
		pass.Reportf(call.Pos(), "использование встроенной функции panic")
	}
}

// checkForbiddenCall сообщает о вызове log.Fatal* или os.Exit вне функции main пакета main.
func checkForbiddenCall(pass *analysis.Pass, call *ast.CallExpr, mainBody *ast.BlockStmt) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}

	isFatal := pkg.Name == "log" && isFatalMethod(sel.Sel.Name)
	isExit := pkg.Name == "os" && sel.Sel.Name == "Exit"

	if !isFatal && !isExit {
		return
	}

	if isInsideBlock(call.Pos(), mainBody) {
		return
	}

	pass.Reportf(call.Pos(), "вызов %s.%s вне функции main пакета main", pkg.Name, sel.Sel.Name)
}

func isFatalMethod(name string) bool {
	return name == "Fatal" || name == "Fatalf" || name == "Fatalln"
}

func isInsideBlock(pos token.Pos, block *ast.BlockStmt) bool {
	if block == nil {
		return false
	}
	return pos > block.Pos() && pos < block.End()
}
