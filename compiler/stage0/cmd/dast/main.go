package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"

	"dastlang/internal/ast"
	"dastlang/internal/compile"
	"dastlang/internal/diag"
	"dastlang/internal/interp"
	"dastlang/internal/ir"
	"dastlang/internal/loader"
	"dastlang/internal/macroexpand"
	"dastlang/internal/qbe"
	"dastlang/internal/source"
	"dastlang/internal/typecheck"
)

func main() {
	applyMemLimit()
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	switch cmd {
	case "build":
		buildCmd(os.Args[2:])
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
	case "ir-qbe":
		emitQBE(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		printStage0Error("", 0, 0, fmt.Sprintf("unknown command: %s", cmd))
		usage()
		os.Exit(1)
	}
}

func applyMemLimit() {
	limit := parseMemLimitBytes()
	if limit <= 0 {
		return
	}
	debug.SetMemoryLimit(limit)
}

func parseMemLimitBytes() int64 {
	if v := os.Getenv("DAST_MEM_LIMIT_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	if v := os.Getenv("DAST_MEM_LIMIT_MB"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n * 1024 * 1024
		}
	}
	return 0
}

func usage() {
	fmt.Fprintln(os.Stderr, "dast - stage0 prototype")
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  dast build [--emit-ir|--emit-qbe] <dir|file.dast ...> [-o output]")
	fmt.Fprintln(os.Stderr, "  dast run <file.dast> [more.dast ...] [-- args...]")
	fmt.Fprintln(os.Stderr, "  dast test [dir|file.dast ...]")
	fmt.Fprintln(os.Stderr, "  dast ir <file.dast> [more.dast ...]")
	fmt.Fprintln(os.Stderr, "  dast ir-run <file.ir> [-- args...]")
	fmt.Fprintln(os.Stderr, "  dast ir-verify <file.ir>")
	fmt.Fprintln(os.Stderr, "  dast ir-opt <file.ir>")
	fmt.Fprintln(os.Stderr, "  dast ir-qbe <file.ir>")
}

func run(args []string) {
	if len(args) < 1 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	files, progArgs, opts := parseLoadArgs(args)
	if len(files) == 0 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	irProg := buildProgram(files, loader.LoadNormal, nil, opts)
	if irProg == nil {
		return
	}
	if err := buildAndRun(irProg, progArgs); err != nil {
		exitOnExecErr(err)
	}
}

func dumpIR(args []string) {
	if len(args) < 1 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	files, _, opts := parseLoadArgs(args)
	if len(files) == 0 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	irProg := buildProgram(files, loader.LoadNormal, nil, opts)
	if irProg == nil {
		return
	}
	fmt.Print(irProg.Format())
}

func testCmd(args []string) {
	files := []string{"."}
	opts := loader.LoadOptions{}
	if len(args) > 0 {
		var progArgs []string
		files, progArgs, opts = parseLoadArgs(args)
		_ = progArgs
		if len(files) == 0 {
			files = []string{"."}
		}
	}
	var entry string
	irProg := buildProgram(files, loader.LoadTest, &entry, opts)
	if irProg == nil {
		return
	}
	if entry != "" {
		fmt.Fprintf(os.Stderr, "[test] entry %s\n", entry)
	} else {
		fmt.Fprintln(os.Stderr, "[test] entry <none>")
	}
	if entry != "" {
		irProg.Entry = entry
	}
	fmt.Fprintln(os.Stderr, "[test] run start")
	if err := buildAndRun(irProg, nil); err != nil {
		fmt.Fprintf(os.Stderr, "[test] run error: %v\n", err)
		exitOnExecErr(err)
	}
	fmt.Fprintln(os.Stderr, "[test] run ok")
}

func buildCmd(args []string) {
	if len(args) < 1 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	outPath := ""
	files := []string{}
	emitIR := false
	emitQBE := false
	opts := loader.LoadOptions{AllowMultiDir: true}
	for i := 0; i < len(args); i++ {
		if args[i] == "--help" || args[i] == "-h" {
			usage()
			return
		}
		if args[i] == "-o" && i+1 < len(args) {
			outPath = args[i+1]
			i++
			continue
		}
		if args[i] == "--emit-ir" {
			emitIR = true
			continue
		}
		if args[i] == "--emit-qbe" {
			emitQBE = true
			continue
		}
		if args[i] == "--bootstrap" {
			opts.AllowMultiDir = true
			continue
		}
		files = append(files, args[i])
	}
	if len(files) == 0 {
		printStage0Error("", 0, 0, "missing input file")
		usage()
		os.Exit(1)
	}
	if emitIR && emitQBE {
		printStage0Error("", 0, 0, "cannot use --emit-ir and --emit-qbe together")
		usage()
		os.Exit(1)
	}
	irProg := buildProgram(files, loader.LoadNormal, nil, opts)
	if irProg == nil {
		return
	}
	if emitIR {
		if outPath != "" {
			if err := os.WriteFile(outPath, []byte(irProg.Format()), 0644); err != nil {
				printStage0Error(outPath, 0, 0, fmt.Sprintf("write failed: %v", err))
				os.Exit(1)
			}
			return
		}
		fmt.Print(irProg.Format())
		return
	}
	if emitQBE {
		out := qbe.EmitProgram(irProg)
		if outPath != "" {
			if err := os.WriteFile(outPath, []byte(out), 0644); err != nil {
				printStage0Error(outPath, 0, 0, fmt.Sprintf("write failed: %v", err))
				os.Exit(1)
			}
			return
		}
		fmt.Print(out)
		return
	}
	if outPath == "" {
		outPath = "a.out"
	}
	if err := writeExecutable(outPath, irProg); err != nil {
		printStage0Error(outPath, 0, 0, fmt.Sprintf("build failed: %v", err))
		os.Exit(1)
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

func emitQBE(args []string) {
	if len(args) != 1 {
		printStage0Error("", 0, 0, "ir-qbe expects exactly one .ir file")
		usage()
		os.Exit(1)
	}
	text, err := os.ReadFile(args[0])
	if err != nil {
		printStage0Error("", 0, 0, err.Error())
		os.Exit(1)
	}
	prog, err := ir.Parse(string(text))
	if err != nil {
		printStage0Error("", 0, 0, err.Error())
		os.Exit(1)
	}
	if err := prog.Validate(); err != nil {
		printStage0Error("", 0, 0, err.Error())
		os.Exit(1)
	}
	out := qbe.EmitProgram(prog)
	fmt.Print(out)
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

func parseLoadArgs(args []string) ([]string, []string, loader.LoadOptions) {
	files, progArgs := splitArgs(args)
	opts := loader.LoadOptions{AllowMultiDir: true}
	if len(files) == 0 {
		return files, progArgs, opts
	}
	out := []string{}
	for _, f := range files {
		if f == "--bootstrap" {
			opts.AllowMultiDir = true
			continue
		}
		out = append(out, f)
	}
	return out, progArgs, opts
}

func writeExecutable(outPath string, prog *ir.Program) error {
	moduleRoot, err := findStage0ModuleRoot()
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(moduleRoot, ".dast-build-")
	if err != nil {
		return err
	}
	if os.Getenv("DAST_KEEP_TMP") == "" {
		defer os.RemoveAll(tmpDir)
	}

	qbePath := filepath.Join(tmpDir, "main.qbe")
	asmPath := filepath.Join(tmpDir, "main.s")
	entryPath := filepath.Join(tmpDir, "entry.c")

	qbeText := qbe.EmitProgram(prog)
	if err := os.WriteFile(qbePath, []byte(qbeText), 0644); err != nil {
		return err
	}

	qbeBin, err := exec.LookPath("qbe")
	if err != nil {
		return fmt.Errorf("qbe not found in PATH")
	}
	cmd := exec.Command(qbeBin, "-o", asmPath, qbePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	entrySrc, err := buildEntryC(prog)
	if err != nil {
		return err
	}
	if err := os.WriteFile(entryPath, []byte(entrySrc), 0644); err != nil {
		return err
	}

	ccBin, err := exec.LookPath("cc")
	if err != nil {
		ccBin, err = exec.LookPath("clang")
		if err != nil {
			return fmt.Errorf("cc/clang not found in PATH")
		}
	}

	runtimePath := filepath.Join(moduleRoot, "runtime", "c_runtime.c")
	if !filepath.IsAbs(outPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		outPath = filepath.Join(cwd, outPath)
	}

	cmd = exec.Command(ccBin, "-O2", "-std=c99", asmPath, runtimePath, entryPath, "-o", outPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildAndRun(prog *ir.Program, progArgs []string) error {
	moduleRoot, err := findStage0ModuleRoot()
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(moduleRoot, ".dast-run-")
	if err != nil {
		return err
	}
	if os.Getenv("DAST_KEEP_TMP") == "" {
		defer os.RemoveAll(tmpDir)
	} else {
		fmt.Fprintln(os.Stderr, "[test] tmp", tmpDir)
	}

	binPath := filepath.Join(tmpDir, "dast-run")
	if err := writeExecutable(binPath, prog); err != nil {
		return err
	}

	cmd := exec.Command(binPath, progArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func buildEntryC(prog *ir.Program) (string, error) {
	entry := prog.Entry
	if entry == "" {
		return "", fmt.Errorf("missing entry function")
	}
	fn, ok := prog.Functions[entry]
	if !ok {
		return "", fmt.Errorf("entry function not found: %s", entry)
	}
	retType := cTypeForIRType(fn.ReturnType)
	entrySym := "dast_user_" + mangleFuncName(entry)
	if retType == "void" {
		return fmt.Sprintf(`#include <stdint.h>

void dast_set_args(int argc, char **argv);
void %s(void);

int main(int argc, char **argv) {
	dast_set_args(argc - 1, argv + 1);
	%s();
	return 0;
}
`, entrySym, entrySym), nil
	}
	return fmt.Sprintf(`#include <stdint.h>

void dast_set_args(int argc, char **argv);
%s %s(void);

int main(int argc, char **argv) {
	dast_set_args(argc - 1, argv + 1);
	return (int)%s();
}
`, retType, entrySym, entrySym), nil
}

func cTypeForIRType(t string) string {
	switch strings.TrimSpace(t) {
	case "", "unit":
		return "void"
	case "bool":
		return "int32_t"
	case "i8":
		return "int8_t"
	case "i16":
		return "int16_t"
	case "u8":
		return "uint8_t"
	case "u16":
		return "uint16_t"
	case "i32", "char":
		return "int32_t"
	case "u32":
		return "uint32_t"
	case "i64", "int", "isize", "usize":
		return "int64_t"
	case "u64":
		return "uint64_t"
	default:
		return "int64_t"
	}
}

func mangleFuncName(name string) string {
	if name == "" {
		return "_"
	}
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}

func findStage0ModuleRoot() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}
	dir := filepath.Dir(exePath)
	for {
		if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found near %s", exePath)
}

type testInfo struct {
	name string
	span source.Span
}

func loadProgram(paths []string, mode loader.LoadMode, opts loader.LoadOptions) *ast.Program {
	prog, diags, err := loader.LoadProgram(paths, mode, opts)
	if err != nil {
		printStage0Error("", 0, 0, err.Error())
		os.Exit(1)
	}
	if exitOnDiag(diags) {
		return nil
	}
	return prog
}

func buildProgram(paths []string, mode loader.LoadMode, testEntry *string, opts loader.LoadOptions) *ir.Program {
	prog := loadProgram(paths, mode, opts)
	if prog == nil {
		return nil
	}
	if exitOnDiag(macroexpand.Expand(prog)) {
		return nil
	}
	if mode == loader.LoadTest {
		tests, diags := collectTests(prog)
		if exitOnDiag(diags) {
			return nil
		}
		if len(tests) == 0 {
			fmt.Fprintln(os.Stderr, "[test] no tests found")
		} else {
			fmt.Fprintf(os.Stderr, "[test] count %d\n", len(tests))
			for _, t := range tests {
				fmt.Fprintf(os.Stderr, "[test] %s\n", t.name)
			}
		}
		testMain, diagMain := buildTestMain(prog, tests)
		if exitOnDiag(diagMain) {
			return nil
		}
		prog.Items = append(prog.Items, testMain)
		if testEntry != nil {
			*testEntry = testMain.Name
		}
	}
	monoProg, diags := typecheck.CheckAndMonomorph(prog)
	if exitOnDiag(diags) {
		return nil
	}
	if monoProg != nil {
		prog = monoProg
	}
	irProg, diags := compile.Compile(prog)
	if exitOnDiag(diags) {
		return nil
	}
	if err := irProg.Validate(); err != nil {
		printStage0Error("<ir>", 0, 0, err.Error())
		os.Exit(1)
	}
	return irProg
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
		startArgs := []ast.Expr{
			&ast.StringLit{Value: "test", SpanInfo: t.span},
			&ast.StringLit{Value: t.name, SpanInfo: t.span},
		}
		startCall := &ast.CallExpr{Callee: "println", Args: startArgs, SpanInfo: t.span}
		startStmt := &ast.ExprStmt{Expr: startCall, SpanInfo: t.span}
		block.Stmts = append(block.Stmts, startStmt)
		call := &ast.CallExpr{Callee: t.name, Args: nil, SpanInfo: t.span}
		stmt := &ast.ExprStmt{Expr: call, SpanInfo: t.span}
		block.Stmts = append(block.Stmts, stmt)
		okArgs := []ast.Expr{
			&ast.StringLit{Value: "ok", SpanInfo: t.span},
			&ast.StringLit{Value: t.name, SpanInfo: t.span},
		}
		okCall := &ast.CallExpr{Callee: "println", Args: okArgs, SpanInfo: t.span}
		okStmt := &ast.ExprStmt{Expr: okCall, SpanInfo: t.span}
		block.Stmts = append(block.Stmts, okStmt)
	}
	okArgs := []ast.Expr{
		&ast.StringLit{Value: "ok", SpanInfo: span},
		&ast.IntLit{Value: int64(len(tests)), SpanInfo: span},
	}
	okCall := &ast.CallExpr{Callee: "println", Args: okArgs, SpanInfo: span}
	okStmt := &ast.ExprStmt{Expr: okCall, SpanInfo: span}
	block.Stmts = append(block.Stmts, okStmt)
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

func exitOnExecErr(err error) {
	if err == nil {
		return
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
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
