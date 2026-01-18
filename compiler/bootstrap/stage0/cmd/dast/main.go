package main

import (
	"fmt"
	"os"

	"dastlang/internal/compile"
	"dastlang/internal/diag"
	"dastlang/internal/interp"
	"dastlang/internal/parser"
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
	case "ir":
		dumpIR(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "dast - stage0 prototype")
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  dast run <file.dast> [more.dast ...] [-- args...]")
	fmt.Fprintln(os.Stderr, "  dast ir <file.dast> [more.dast ...]")
}

func run(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "missing input file")
		usage()
		os.Exit(1)
	}
	files, progArgs := splitArgs(args)
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "missing input file")
		usage()
		os.Exit(1)
	}
	sources := make([]parser.Source, 0, len(files))
	for _, filename := range files {
		src, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", filename, err)
			os.Exit(1)
		}
		sources = append(sources, parser.Source{Filename: filename, Input: string(src)})
	}
	prog, diags := parser.ParseFiles(sources)
	if exitOnDiag(diags) {
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rt := interp.New(irProg)
	if len(progArgs) > 0 {
		rt.Args = progArgs
	}
	if _, err := rt.Run(irProg.Entry); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func dumpIR(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "missing input file")
		usage()
		os.Exit(1)
	}
	files, _ := splitArgs(args)
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "missing input file")
		usage()
		os.Exit(1)
	}
	sources := make([]parser.Source, 0, len(files))
	for _, filename := range files {
		src, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", filename, err)
			os.Exit(1)
		}
		sources = append(sources, parser.Source{Filename: filename, Input: string(src)})
	}
	prog, diags := parser.ParseFiles(sources)
	if exitOnDiag(diags) {
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
		fmt.Fprintln(os.Stderr, err)
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
