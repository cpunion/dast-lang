package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"dastlang/internal/ast"
	"dastlang/internal/compile"
	"dastlang/internal/diag"
	"dastlang/internal/interp"
	"dastlang/internal/ir"
	"dastlang/internal/loader"
	"dastlang/internal/source"
	"dastlang/internal/typecheck"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	switch cmd {
	case "run":
		run(os.Args[2:])
	case "test":
		testCmd(os.Args[2:])
	case "ir":
		dumpIR(os.Args[2:])
	case "ir-run":
		runIR(os.Args[2:])
	case "ir-verify":
		verifyIR(os.Args[2:])
	case "ir-opt":
		optIR(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		printStage0Error("", 0, 0, fmt.Sprintf("unknown command: %s", cmd))
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "dast - stage0 prototype")
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  dast run <file.dast> [more.dast ...] [-- args...]")
	fmt.Fprintln(os.Stderr, "  dast test [dir|file.dast ...]")
	fmt.Fprintln(os.Stderr, "  dast ir <file.dast> [more.dast ...]")
	fmt.Fprintln(os.Stderr, "  dast ir-run <file.ir> [-- args...]")
	fmt.Fprintln(os.Stderr, "  dast ir-verify <file.ir>")
	fmt.Fprintln(os.Stderr, "  dast ir-opt <file.ir>")
}

func run(args []string) {
	if len(args) < 1 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	files, progArgs := splitArgs(args)
	if len(files) == 0 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	prog := loadProgram(files, loader.LoadNormal)
	if prog == nil {
		return
	}
	if exitOnDiag(typecheck.Check(prog)) {
		return
	}
	irProg, diags := compile.Compile(prog)
	if exitOnDiag(diags) {
		return
	}
	if err := irProg.Validate(); err != nil {
		printStage0Error("<ir>", 0, 0, err.Error())
		os.Exit(1)
	}
	rt := interp.New(irProg)
	if len(progArgs) > 0 {
		rt.Args = progArgs
	}
	if _, err := rt.Run(irProg.Entry); err != nil {
		exitOnRunErr(err)
	}
}

func dumpIR(args []string) {
	if len(args) < 1 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	files, _ := splitArgs(args)
	if len(files) == 0 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	prog := loadProgram(files, loader.LoadNormal)
	if prog == nil {
		return
	}
	if exitOnDiag(typecheck.Check(prog)) {
		return
	}
	irProg, diags := compile.Compile(prog)
	if exitOnDiag(diags) {
		return
	}
	if err := irProg.Validate(); err != nil {
		printStage0Error("<ir>", 0, 0, err.Error())
		os.Exit(1)
	}
	fmt.Print(irProg.Format())
}

func testCmd(args []string) {
	files := []string{"."}
	if len(args) > 0 {
		files, _ = splitArgs(args)
		if len(files) == 0 {
			files = []string{"."}
		}
	}
	prog := loadProgram(files, loader.LoadTest)
	if prog == nil {
		return
	}
	tests, diags := collectTests(prog)
	if exitOnDiag(diags) {
		return
	}
	if len(tests) == 0 {
		return
	}
	testMain, diagMain := buildTestMain(prog, tests)
	if exitOnDiag(diagMain) {
		return
	}
	prog.Items = append(prog.Items, testMain)
	if exitOnDiag(typecheck.Check(prog)) {
		return
	}
	irProg, diags := compile.Compile(prog)
	if exitOnDiag(diags) {
		return
	}
	if err := irProg.Validate(); err != nil {
		printStage0Error("<ir>", 0, 0, err.Error())
		os.Exit(1)
	}
	irProg.Entry = testMain.Name
	rt := interp.New(irProg)
	if _, err := rt.Run(irProg.Entry); err != nil {
		exitOnRunErr(err)
	}
}

func runIR(args []string) {
	if len(args) < 1 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	files, progArgs := splitArgs(args)
	if len(files) != 1 {
		printStage0Error("", 0, 0, "ir-run expects exactly one .ir file")
		usage()
		os.Exit(1)
	}
	src, err := os.ReadFile(files[0])
	if err != nil {
		printStage0Error(files[0], 0, 0, fmt.Sprintf("read failed: %v", err))
		os.Exit(1)
	}
	irProg, err := ir.Parse(string(src))
	if err != nil {
		printIRParseError(files[0], err)
		os.Exit(1)
	}
	if err := irProg.Validate(); err != nil {
		printStage0Error(files[0], 0, 0, err.Error())
		os.Exit(1)
	}
	rt := interp.New(irProg)
	if len(progArgs) > 0 {
		rt.Args = progArgs
	}
	if _, err := rt.Run(irProg.Entry); err != nil {
		exitOnRunErr(err)
	}
}

func verifyIR(args []string) {
	if len(args) < 1 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	files, _ := splitArgs(args)
	if len(files) != 1 {
		printStage0Error("", 0, 0, "ir-verify expects exactly one .ir file")
		usage()
		os.Exit(1)
	}
	src, err := os.ReadFile(files[0])
	if err != nil {
		printStage0Error(files[0], 0, 0, fmt.Sprintf("read failed: %v", err))
		os.Exit(1)
	}
	irProg, err := ir.Parse(string(src))
	if err != nil {
		printIRParseError(files[0], err)
		os.Exit(1)
	}
	if err := irProg.Validate(); err != nil {
		printStage0Error(files[0], 0, 0, err.Error())
		os.Exit(1)
	}
}

func optIR(args []string) {
	if len(args) < 1 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	files, _ := splitArgs(args)
	if len(files) != 1 {
		printStage0Error("", 0, 0, "ir-opt expects exactly one .ir file")
		usage()
		os.Exit(1)
	}
	src, err := os.ReadFile(files[0])
	if err != nil {
		printStage0Error(files[0], 0, 0, fmt.Sprintf("read failed: %v", err))
		os.Exit(1)
	}
	irProg, err := ir.Parse(string(src))
	if err != nil {
		printIRParseError(files[0], err)
		os.Exit(1)
	}
	if err := irProg.Validate(); err != nil {
		printStage0Error(files[0], 0, 0, err.Error())
		os.Exit(1)
	}
	ir.Optimize(irProg)
	if err := irProg.Validate(); err != nil {
		printStage0Error(files[0], 0, 0, err.Error())
		os.Exit(1)
	}
	fmt.Print(irProg.Format())
}

func splitArgs(args []string) ([]string, []string) {
	files := []string{}
	progArgs := []string{}
	for i, arg := range args {
		if arg == "--" {
			progArgs = args[i+1:]
			break
		}
		files = append(files, arg)
	}
	return files, progArgs
}

type testInfo struct {
	name string
	span source.Span
}

func loadProgram(paths []string, mode loader.LoadMode) *ast.Program {
	prog, diags, err := loader.LoadProgram(paths, mode)
	if err != nil {
		printStage0Error("", 0, 0, err.Error())
		os.Exit(1)
	}
	if exitOnDiag(diags) {
		return nil
	}
	return prog
}

func collectTests(prog *ast.Program) ([]testInfo, *diag.Bag) {
	if prog == nil {
		return nil, nil
	}
	diags := &diag.Bag{}
	var tests []testInfo
	for _, item := range prog.Items {
		fn, ok := item.(*ast.Function)
		if !ok {
			continue
		}
		if !strings.HasPrefix(fn.Name, "test_") {
			continue
		}
		file := fn.Span().Start.Filename
		if !strings.HasSuffix(file, "_test.dast") {
			continue
		}
		if len(fn.Params) != 0 {
			diags.Add(fn.Span(), "test function must not accept parameters")
			continue
		}
		tests = append(tests, testInfo{name: fn.Name, span: fn.Span()})
	}
	sort.Slice(tests, func(i, j int) bool {
		return tests[i].name < tests[j].name
	})
	if len(diags.Items) == 0 {
		return tests, nil
	}
	return tests, diags
}

func buildTestMain(prog *ast.Program, tests []testInfo) (*ast.Function, *diag.Bag) {
	diags := &diag.Bag{}
	const name = "__dast_test_main"
	for _, item := range prog.Items {
		if fn, ok := item.(*ast.Function); ok && fn.Name == name {
			diags.Add(fn.Span(), "test entry function already defined")
			break
		}
	}
	if len(diags.Items) > 0 {
		return nil, diags
	}
	span := source.Span{}
	if len(tests) > 0 {
		span = tests[0].span
	}
	block := &ast.Block{SpanInfo: span}
	for _, t := range tests {
		call := &ast.CallExpr{Callee: t.name, Args: nil, SpanInfo: t.span}
		stmt := &ast.ExprStmt{Expr: call, SpanInfo: t.span}
		block.Stmts = append(block.Stmts, stmt)
	}
	fn := &ast.Function{Name: name, Body: block, SpanInfo: span}
	return fn, nil
}

func exitOnDiag(diags *diag.Bag) bool {
	if diags == nil {
		return false
	}
	if diags.HasErrors() {
		fmt.Fprintln(os.Stderr, diags.Error())
		os.Exit(1)
		return true
	}
	return false
}

func exitOnRunErr(err error) {
	if err == nil {
		return
	}
	var exitErr interp.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.Code)
	}
	printStage0Error("<runtime>", 0, 0, err.Error())
	os.Exit(1)
}

func printStage0Error(file string, line, col int, msg string) {
	if file == "" {
		file = "<unknown>"
	}
	fmt.Fprintf(os.Stderr, "stage0: %s:%d:%d: error %s\n", file, line, col, msg)
}

func printIRParseError(file string, err error) {
	msg := err.Error()
	line, detail, ok := parseIRParseError(msg)
	if ok {
		printStage0Error(file, line, 1, detail)
		return
	}
	printStage0Error(file, 0, 0, msg)
}

func parseIRParseError(msg string) (int, string, bool) {
	const prefix = "ir parse error (line "
	if !strings.HasPrefix(msg, prefix) {
		return 0, "", false
	}
	rest := msg[len(prefix):]
	end := strings.Index(rest, "): ")
	if end < 0 {
		return 0, "", false
	}
	lineStr := rest[:end]
	line, err := strconv.Atoi(lineStr)
	if err != nil {
		return 0, "", false
	}
	return line, rest[end+3:], true
}
