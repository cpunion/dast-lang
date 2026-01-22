package interp

import (
	"errors"
	"fmt"
	"io"
	"os"

	"dastlang/internal/ir"
)

type Builtin func(args []ir.Value) (ir.Value, error)

type ExitError struct {
	Code int
}

func (e ExitError) Error() string {
	return fmt.Sprintf("exit %d", e.Code)
}

type Runtime struct {
	Prog       *ir.Program
	Builtins   map[string]Builtin
	Stdout     io.Writer
	Args       []string
	nextAddr   int
	heap       map[int]*ir.Value
	instrCount int64
	Debug      bool
}

func New(prog *ir.Program) *Runtime {
	rt := &Runtime{Prog: prog, Stdout: os.Stdout, heap: map[int]*ir.Value{}, Debug: os.Getenv("DAST_DEBUG") != ""}
	rt.Builtins = map[string]Builtin{
		"print":      rt.builtinPrint(false),
		"println":    rt.builtinPrint(true),
		"len":        rt.builtinLen(),
		"push":       rt.builtinPush(),
		"exit":       rt.builtinExit(),
		"read_file":  rt.builtinReadFile(),
		"read_dir":   rt.builtinReadDir(),
		"write_file": rt.builtinWriteFile(),
		"mkdir":      rt.builtinMkdir(),
		"args":       rt.builtinArgs(),
		"char_at":    rt.builtinCharAt(),
		"substr":     rt.builtinSubstr(),
		"pop":        rt.builtinPop(),
		"read_line":  rt.builtinReadLine(),
		"read_bytes": rt.builtinReadBytes(),
	}
	return rt
}

func (rt *Runtime) Run(entry string) (ir.Value, error) {
	if entry == "" {
		entry = rt.Prog.Entry
	}
	if entry == "" {
		return ir.Value{Kind: ir.KindUnit}, errors.New("no entry function")
	}
	return rt.callFunction(entry, nil)
}

type frame struct {
	fn       *ir.Function
	vars     map[string]*ir.Value
	varAddrs map[string]int
	temps    []ir.Value
	blocks   map[string]*ir.Block
	cur      *ir.Block
}

func (rt *Runtime) callFunction(name string, args []ir.Value) (ir.Value, error) {
	if builtin, ok := rt.Builtins[name]; ok {
		return builtin(args)
	}
	fn, ok := rt.Prog.Functions[name]
	if !ok {
		return ir.Value{Kind: ir.KindUnit}, fmt.Errorf("undefined function '%s'", name)
	}
	if len(args) != len(fn.Params) {
		return ir.Value{Kind: ir.KindUnit}, fmt.Errorf("function '%s' expects %d args, got %d", name, len(fn.Params), len(args))
	}
	fr := &frame{
		fn:       fn,
		vars:     map[string]*ir.Value{},
		varAddrs: map[string]int{},
		temps:    make([]ir.Value, fn.TempCount),
		blocks:   map[string]*ir.Block{},
	}
	for _, b := range fn.Blocks {
		fr.blocks[b.Label] = b
	}
	if len(fn.Blocks) == 0 {
		return ir.Value{Kind: ir.KindUnit}, nil
	}
	fr.cur = fn.Blocks[0]
	// Params are now temp IDs (t0, t1...), store them in temps array
	for i, val := range args {
		if err := rt.setTemp(fr, i, val); err != nil {
			return ir.Value{Kind: ir.KindUnit}, err
		}
	}
	return rt.execFrame(fr)
}

func (rt *Runtime) execFrame(fr *frame) (ir.Value, error) {
	for {
		blk := fr.cur
		if blk == nil {
			return ir.Value{Kind: ir.KindUnit}, errors.New("runtime error: nil block")
		}
		for _, inst := range blk.Instr {
			rt.instrCount++
			if rt.Debug && rt.instrCount%100000 == 0 {
				fmt.Fprintf(os.Stderr, "[DEBUG] %d instructions, fn=%s, block=%s\n", rt.instrCount, fr.fn.Name, blk.Label)
			}
			if err := rt.execInstr(fr, inst); err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
		}
		if blk.Term == nil {
			return ir.Value{Kind: ir.KindUnit}, nil
		}
		switch t := blk.Term.(type) {
		case *ir.Jump:
			next, ok := fr.blocks[t.Target]
			if !ok {
				return ir.Value{Kind: ir.KindUnit}, fmt.Errorf("unknown block '%s'", t.Target)
			}
			fr.cur = next
			continue
		case *ir.Branch:
			cond, err := rt.getTemp(fr, t.Cond)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			if cond.Kind != ir.KindBool {
				return ir.Value{Kind: ir.KindUnit}, errors.New("branch condition must be bool")
			}
			target := t.Else
			if cond.Bool {
				target = t.Then
			}
			next, ok := fr.blocks[target]
			if !ok {
				return ir.Value{Kind: ir.KindUnit}, fmt.Errorf("unknown block '%s'", target)
			}
			fr.cur = next
			continue
		case *ir.Return:
			if t.Value == nil {
				return ir.Value{Kind: ir.KindUnit}, nil
			}
			val, err := rt.getTemp(fr, *t.Value)
			if err != nil {
				return ir.Value{Kind: ir.KindUnit}, err
			}
			return val, nil
		default:
			return ir.Value{Kind: ir.KindUnit}, errors.New("unknown terminator")
		}
	}
}

func (rt *Runtime) allocAddr(ptr *ir.Value) int {
	addr := rt.nextAddr
	rt.nextAddr++
	rt.heap[addr] = ptr
	return addr
}
