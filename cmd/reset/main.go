package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

const generateDirective = "// generate:reset"

// primitiveZeroValues сопоставляет имена встроенных примитивных типов с их нулевыми значениями.
var primitiveZeroValues = map[string]string{
	"int":        "0",
	"int8":       "0",
	"int16":      "0",
	"int32":      "0",
	"int64":      "0",
	"uint":       "0",
	"uint8":      "0",
	"uint16":     "0",
	"uint32":     "0",
	"uint64":     "0",
	"uintptr":    "0",
	"float32":    "0",
	"float64":    "0",
	"complex64":  "0",
	"complex128": "0",
	"byte":       "0",
	"rune":       "0",
	"bool":       "false",
	"string":     `""`,
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	if err := run(root); err != nil {
		fmt.Fprintln(os.Stderr, "reset generator:", err)
		os.Exit(1)
	}
}

// run обходит дерево директорий и генерирует методы Reset() для аннотированных структур.
func run(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		// Пропускаем скрытые директории, vendor и testdata, но не корневую директорию.
		if base != "." && (strings.HasPrefix(base, ".") || base == "vendor" || base == "testdata") {
			return filepath.SkipDir
		}
		return processDir(path)
	})
}

// processDir парсит все не-тестовые Go-файлы в директории и записывает reset.gen.go,
// если найдены структуры с директивой generate:reset.
func processDir(dir string) error {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		n := fi.Name()
		return !strings.HasSuffix(n, "_test.go") && n != "reset.gen.go"
	}, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", dir, err)
	}

	for pkgName, pkg := range pkgs {
		var methods []string
		for _, file := range pkg.Files {
			methods = append(methods, collectMethods(file)...)
		}
		if len(methods) == 0 {
			continue
		}

		outPath := filepath.Join(dir, "reset.gen.go")
		if err := os.WriteFile(outPath, buildFile(pkgName, methods), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		fmt.Println("generated:", outPath)
	}
	return nil
}

// collectMethods возвращает сгенерированные тела методов Reset() для всех
// аннотированных структур в файле.
func collectMethods(file *ast.File) []string {
	var methods []string
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		// Директива может стоять над GenDecl или над отдельным TypeSpec (в группе типов).
		declAnnotated := hasDirective(genDecl.Doc)
		for _, spec := range genDecl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			if !declAnnotated && !hasDirective(ts.Doc) {
				continue
			}
			methods = append(methods, buildResetMethod(ts.Name.Name, st))
		}
	}
	return methods
}

// hasDirective проверяет, содержит ли группа комментариев директиву generate:reset.
func hasDirective(cg *ast.CommentGroup) bool {
	if cg == nil {
		return false
	}
	for _, c := range cg.List {
		if strings.TrimSpace(c.Text) == generateDirective {
			return true
		}
	}
	return false
}

// buildResetMethod генерирует полный исходный текст метода Reset() для структуры.
func buildResetMethod(name string, st *ast.StructType) string {
	recv := strings.ToLower(string(name[0]))
	var b strings.Builder

	fmt.Fprintf(&b, "func (%s *%s) Reset() {\n", recv, name)
	fmt.Fprintf(&b, "if %s == nil {\nreturn\n}\n\n", recv)

	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			// Встроенное (анонимное) поле: имя поля совпадает с именем типа.
			embName := embeddedFieldName(field.Type)
			if embName == "" {
				continue
			}
			for _, line := range resetField(recv, embName, field.Type) {
				fmt.Fprintln(&b, line)
			}
			continue
		}
		for _, ident := range field.Names {
			for _, line := range resetField(recv, ident.Name, field.Type) {
				fmt.Fprintln(&b, line)
			}
		}
	}

	fmt.Fprintln(&b, "}")
	return b.String()
}

// embeddedFieldName возвращает неквалифицированное имя для доступа к встроенному полю.
func embeddedFieldName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return embeddedFieldName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}

// resetField генерирует инструкции сброса для одного именованного поля структуры.
func resetField(recv, field string, typ ast.Expr) []string {
	ref := recv + "." + field

	switch t := typ.(type) {
	case *ast.Ident:
		if zero, ok := primitiveZeroValues[t.Name]; ok {
			return []string{ref + " = " + zero}
		}
		// Именованный непримитивный тип: вызываем Reset() через интерфейс, если доступен.
		return tryResetValue(ref)

	case *ast.StarExpr:
		return resetPointer(ref, t.X)

	case *ast.ArrayType:
		if t.Len == nil {
			// Слайс: обрезаем до нулевой длины, сохраняя базовый массив.
			return []string{fmt.Sprintf("%s = %s[:0]", ref, ref)}
		}
		// Массив фиксированного размера: не описан в спецификации, пропускаем.
		return nil

	case *ast.MapType:
		return []string{fmt.Sprintf("clear(%s)", ref)}

	case *ast.SelectorExpr:
		// Квалифицированное имя из другого пакета: вызываем Reset(), если доступен.
		return tryResetValue(ref)

	default:
		return nil
	}
}

// resetPointer генерирует инструкции сброса для поля с типом-указателем.
func resetPointer(ref string, pointed ast.Expr) []string {
	switch t := pointed.(type) {
	case *ast.Ident:
		if zero, ok := primitiveZeroValues[t.Name]; ok {
			// *примитив: разыменовываем и присваиваем нулевое значение.
			return []string{
				fmt.Sprintf("if %s != nil {", ref),
				fmt.Sprintf("*%s = %s", ref, zero),
				"}",
			}
		}
		// *ИменованныйТип: вызываем Reset() через интерфейс, если доступен.
		return tryResetPointer(ref)

	case *ast.SelectorExpr:
		// *pkg.Type
		return tryResetPointer(ref)

	case *ast.ArrayType:
		if t.Len == nil {
			// *[]T: обрезаем слайс по указателю.
			return []string{
				fmt.Sprintf("if %s != nil {", ref),
				fmt.Sprintf("*%s = (*%s)[:0]", ref, ref),
				"}",
			}
		}
		return nil

	case *ast.MapType:
		// *map[K]V
		return []string{
			fmt.Sprintf("if %s != nil {", ref),
			fmt.Sprintf("clear(*%s)", ref),
			"}",
		}

	default:
		return nil
	}
}

// tryResetValue генерирует вызов Reset() через интерфейс для поля-значения (не указателя).
// Передаём адрес поля, чтобы метод с pointer-receiver был достижим.
func tryResetValue(ref string) []string {
	return []string{
		fmt.Sprintf("if resetter, ok := interface{}(&%s).(interface{ Reset() }); ok {", ref),
		"resetter.Reset()",
		"}",
	}
}

// tryResetPointer генерирует вызов Reset() через интерфейс для поля-указателя.
// Оборачиваем указатель в interface{}, чтобы type assertion компилировался для конкретных типов.
func tryResetPointer(ref string) []string {
	return []string{
		fmt.Sprintf("if resetter, ok := interface{}(%s).(interface{ Reset() }); ok && %s != nil {", ref, ref),
		"resetter.Reset()",
		"}",
	}
}

// buildFile собирает и форматирует через gofmt итоговый сгенерированный файл.
func buildFile(pkgName string, methods []string) []byte {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, "// Code generated by reset generator. DO NOT EDIT.")
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, "package %s\n", pkgName)
	fmt.Fprintln(&buf)
	for _, m := range methods {
		fmt.Fprintln(&buf, m)
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes()
	}
	return src
}
