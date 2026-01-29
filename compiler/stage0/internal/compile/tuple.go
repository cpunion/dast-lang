package compile

import (
	"fmt"

	"dastlang/internal/ast"
	"dastlang/internal/ir"
)

func (c *Compiler) ensureTupleType(elemTypes []string) string {
	name := tupleTypeName(elemTypes)
	if c.prog.TypeDecls != nil {
		if _, ok := c.prog.TypeDecls[name]; ok {
			return name
		}
	}
	fields := make([]ir.Var, 0, len(elemTypes))
	for i, t := range elemTypes {
		fields = append(fields, ir.Var{Name: fmt.Sprintf("%d", i), Type: t})
	}
	c.prog.TypeDecls[name] = &ir.TypeDecl{Name: name, Fields: fields}
	return name
}

func (c *Compiler) registerTupleTypesInType(t ast.Type) {
	if t.IsArray && t.Elem != nil {
		c.registerTupleTypesInType(*t.Elem)
	}
	if t.IsTuple {
		if len(t.TupleElems) == 0 {
			return
		}
		elemTypes := make([]string, 0, len(t.TupleElems))
		for _, e := range t.TupleElems {
			c.registerTupleTypesInType(e)
			elemTypes = append(elemTypes, formatType(e))
		}
		c.ensureTupleType(elemTypes)
	}
	if len(t.Args) > 0 {
		for _, a := range t.Args {
			c.registerTupleTypesInType(a)
		}
	}
}

func (c *Compiler) tupleFieldType(tupleName string, index int) string {
	if c.prog.TypeDecls == nil {
		return ""
	}
	if td, ok := c.prog.TypeDecls[tupleName]; ok {
		if index >= 0 && index < len(td.Fields) {
			return td.Fields[index].Type
		}
	}
	return ""
}
