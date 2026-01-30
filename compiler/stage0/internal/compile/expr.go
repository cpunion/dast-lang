package compile

import (
	"fmt"
	"strings"

	"dastlang/internal/ast"
	"dastlang/internal/ir"
)

// compileOperand compiles an expression and returns an Operand (inline constant or temp)
func (c *Compiler) compileOperand(expr ast.Expr) ir.Operand {
	// For literals, return inline constants
	switch e := expr.(type) {
	case *ast.IntLit:
		return ir.IntOperand(e.Value)
	case *ast.CharLit:
		return ir.ConstOperand(ir.Value{Kind: ir.KindInt, Int: e.Value, IntType: "char"})
	case *ast.FloatLit:
		return ir.FloatOperand(e.Text)
	case *ast.BoolLit:
		return ir.BoolOperand(e.Value)
	case *ast.StringLit:
		return ir.StringOperand(e.Value)
	case *ast.BlockExpr:
		return c.compileBlockExprOperand(e.Block)
	case *ast.IfExpr:
		return ir.TempOperand(c.compileIfExpr(e))
	case *ast.MatchExpr:
		return ir.TempOperand(c.compileMatchExpr(e))
	case *ast.IdentExpr:
		if info, ok := c.consts[e.Name]; ok {
			return ir.ConstOperand(constValueToIr(info.Value, info.TypeName))
		}
	case *ast.CompileExpr:
		if c.opts.AllowCompile {
			return c.compileOperand(e.Expr)
		}
		c.diag.Add(e.Span(), "compile! must be expanded before compile")
		return ir.IntOperand(0)
	}
	// For everything else, compile to temp and wrap
	return ir.TempOperand(c.compileExpr(expr))
}

func (c *Compiler) compileOperandBorrow(expr ast.Expr) ir.Operand {
	// "Borrow" means: produce an operand that does not take ownership, so it must
	// not trigger drop of the underlying value. This is required for owned,
	// non-copy values (e.g. String) used as operands to non-consuming operations
	// (calls, comparisons, etc.).
	switch e := expr.(type) {
	case *ast.IdentExpr:
		if varInfo, ok := c.lookupVar(e.Name); ok {
			if c.needsDropType(varInfo.Type) && !isCopyTypeName(varInfo.Type) && !isRefTypeName(varInfo.Type) {
				t := c.compileExpr(expr)
				c.setTempBorrowed(t, true)
				return ir.TempOperand(t)
			}
		}
	}
	return c.compileOperand(expr)
}

func (c *Compiler) compileCallArg(expr ast.Expr, callee string, index int) ir.Operand {
	if callee == "len" {
		argType := c.inferExprType(expr)
		if isRefTypeName(argType) {
			refTemp := c.compileExpr(expr)
			t := c.newTemp()
			typ := derefTypeName(argType)
			if typ == "" {
				typ = "i64"
			}
			c.setTempType(t, typ)
			c.emit(&ir.LoadVar{Dst: t, Ref: true, RefTemp: refTemp})
			// len() borrows its argument. The loaded value is a borrowed view into
			// the referenced storage and must not be dropped.
			c.setTempBorrowed(t, true)
			return ir.TempOperand(t)
		}
	}
	if params, ok := c.funcParamTypes[callee]; ok && index < len(params) && isRefTypeName(params[index]) {
		if isRefTypeName(c.inferExprType(expr)) {
			return c.compileOperandBorrow(expr)
		}
		// Reference parameters borrow the argument's storage.
		return ir.TempOperand(c.compileRefTarget(expr, false))
	}
	if c.callArgConsumes(callee, index) {
		return c.compileOperandMove(expr)
	}
	return c.compileOperandBorrow(expr)
}

func (c *Compiler) compileOperandMove(expr ast.Expr) ir.Operand {
	switch e := expr.(type) {
	case *ast.AccessExpr:
		fieldType := c.inferExprType(expr)
		if fieldType == "" {
			fieldType = "i64"
		}
		if c.needsDropType(fieldType) && !isCopyTypeName(fieldType) {
			return c.compileAccessMove(e, fieldType)
		}
	case *ast.IndexExpr:
		elemType := c.inferExprType(expr)
		if elemType == "" {
			elemType = "i64"
		}
		if c.needsDropType(elemType) && !isCopyTypeName(elemType) {
			return c.compileIndexMove(e, elemType)
		}
	}
	op := c.compileOperand(expr)
	c.markMovedExpr(expr)
	return op
}

func (c *Compiler) compileAccessMove(e *ast.AccessExpr, fieldType string) ir.Operand {
	recv := c.compileExpr(e.Receiver)
	c.markTempBorrowedVar(recv)
	dst := c.newTemp()
	c.setTempType(dst, fieldType)
	c.setTempBorrowed(dst, false)
	c.emit(&ir.GetField{Dst: dst, Src: recv, Field: e.Field})
	// Clear the field to avoid double-drop when the base is dropped later.
	c.emit(&ir.SetField{Src: recv, Field: e.Field, Value: ir.IntOperand(0)})
	return ir.TempOperand(dst)
}

func (c *Compiler) compileIndexMove(e *ast.IndexExpr, elemType string) ir.Operand {
	recv := c.compileOperandBorrow(e.Receiver)
	index := c.compileOperandBorrow(e.Index)
	if !recv.IsConst {
		c.setTempBorrowed(recv.Temp, true)
		c.markTempBorrowedVar(recv.Temp)
	}
	dst := c.newTemp()
	c.setTempType(dst, elemType)
	c.setTempBorrowed(dst, false)
	c.emit(&ir.Index{Dst: dst, Array: recv, Index: index})
	// Clear element to avoid double-drop when the array is dropped later.
	c.emit(&ir.SetIndex{Array: recv, Index: index, Src: ir.IntOperand(0)})
	return ir.TempOperand(dst)
}

func (c *Compiler) compileExpr(expr ast.Expr) int {
	switch e := expr.(type) {
	case *ast.ArrayLit:
		elems := make([]ir.Operand, 0, len(e.Elems))
		for _, elem := range e.Elems {
			elems = append(elems, c.compileOperandMove(elem))
		}
		dst := c.newTemp()
		elemType := "i64"
		if len(e.Elems) > 0 {
			if t := c.inferExprType(e.Elems[0]); t != "" {
				elemType = t
			}
		}
		c.setTempType(dst, "["+elemType+"]")
		c.setTempBorrowed(dst, false)
		c.emit(&ir.MakeArray{Dst: dst, Elems: elems})
		return dst
	case *ast.TupleLit:
		if len(e.Elems) == 0 {
			return c.operandToTemp(ir.ConstOperand(ir.Value{Kind: ir.KindUnit}), "unit")
		}
		elemTypes := make([]string, 0, len(e.Elems))
		fields := make([]ir.StructFieldInit, 0, len(e.Elems))
		for i, elem := range e.Elems {
			op := c.compileOperandMove(elem)
			typ := c.inferExprType(elem)
			if typ == "" {
				typ = "i64"
			}
			elemTypes = append(elemTypes, typ)
			fields = append(fields, ir.StructFieldInit{Name: fmt.Sprintf("%d", i), Src: op})
		}
		tupleName := c.ensureTupleType(elemTypes)
		dst := c.newTemp()
		c.setTempType(dst, tupleName)
		c.setTempBorrowed(dst, false)
		c.emit(&ir.MakeStruct{Dst: dst, Name: tupleName, Fields: fields})
		return dst
	case *ast.IntLit:
		return c.operandToTemp(ir.IntOperand(e.Value), "i64")
	case *ast.CharLit:
		return c.operandToTemp(ir.ConstOperand(ir.Value{Kind: ir.KindInt, Int: e.Value, IntType: "char"}), "char")
	case *ast.BoolLit:
		return c.operandToTemp(ir.BoolOperand(e.Value), "bool")
	case *ast.StringLit:
		return c.operandToTemp(ir.StringOperand(e.Value), "String")
	case *ast.BlockExpr:
		op := c.compileBlockExprOperand(e.Block)
		return c.operandToTemp(op, c.inferExprType(e))
	case *ast.LoopExpr:
		return c.compileLoopExpr(e)
	case *ast.IfExpr:
		return c.compileIfExpr(e)
	case *ast.MatchExpr:
		return c.compileMatchExpr(e)
	case *ast.ClosureExpr:
		return c.compileClosureExpr(e)
	case *ast.IdentExpr:
		varInfo, ok := c.lookupVar(e.Name)
		if !ok {
			if _, ok := c.consts[e.Name]; ok {
				c.diag.Add(e.Span(), fmt.Sprintf("const '%s' cannot be used where a temp is required", e.Name))
				return c.constZero()
			}
			c.diag.Add(e.Span(), fmt.Sprintf("undefined variable '%s'", e.Name))
			return c.constZero()
		}
		if varInfo.Moved && c.needsDropType(varInfo.Type) {
			c.diag.Add(e.Span(), fmt.Sprintf("use of moved value '%s'", e.Name))
			return c.constZero()
		}
		if varInfo.RefTemp >= 0 {
			t := c.newTemp()
			typ := derefTypeName(varInfo.Type)
			if typ == "" {
				typ = "i64"
			}
			c.setTempType(t, typ)
			c.emit(&ir.LoadVar{Dst: t, Ref: true, RefTemp: varInfo.RefTemp})
			c.setTempBorrowed(t, varInfo.Borrowed)
			return t
		}
		if varInfo.Temp >= 0 {
			// Value parameter: return the temp directly
			c.setTempBorrowed(varInfo.Temp, varInfo.Borrowed)
			return varInfo.Temp
		}
		// Memory variable (let or let mut): generate load
		t := c.newTemp()
		typ := varInfo.Type
		if typ == "" {
			typ = "i64"
		}
		c.setTempType(t, typ)
		c.emit(&ir.LoadVar{Dst: t, Name: varInfo.Name})
		c.setTempBorrowed(t, varInfo.Borrowed)
		return t
	case *ast.CompileExpr:
		if c.opts.AllowCompile {
			return c.compileExpr(e.Expr)
		}
		c.diag.Add(e.Span(), "compile! must be expanded before compile")
		return c.constZero()
	case *ast.QuoteExpr:
		return c.compileQuoteExpr(e)
	case *ast.MacroCallExpr:
		c.diag.Add(e.Span(), "macro call must be expanded before compile")
		return c.constZero()
	case *ast.RefExpr:
		return c.compileRefTarget(e.Expr, e.Mutable)
	case *ast.DerefExpr:
		srcType := c.inferExprType(e.Expr)
		if srcType == "str" {
			return c.compileExpr(e.Expr)
		}
		src := c.compileExpr(e.Expr)
		typ := derefTypeName(srcType)
		t := c.newTemp()
		if typ == "" {
			typ = "i64"
		}
		c.setTempType(t, typ)
		c.emit(&ir.LoadVar{Dst: t, Ref: true, RefTemp: src})
		c.setTempBorrowed(t, true)
		return t
	case *ast.UnaryExpr:
		src := c.compileOperandBorrow(e.Expr)
		switch e.Op {
		case "-":
			t := c.newTemp()
			typ := c.inferExprType(e)
			c.setTempType(t, typ)
			c.setTempBorrowed(t, false)
			c.emit(&ir.BinOp{Dst: t, Op: "-", Lhs: ir.IntOperand(0), Rhs: src})
			return t
		case "!":
			t := c.newTemp()
			c.setTempType(t, "bool")
			c.setTempBorrowed(t, false)
			c.emit(&ir.BinOp{Dst: t, Op: "==", Lhs: src, Rhs: ir.BoolOperand(false)})
			return t
		default:
			return c.constZero()
		}
	case *ast.BinaryExpr:
		if e.Op == "&&" || e.Op == "||" {
			thenExpr := e.Right
			elseExpr := ast.Expr(&ast.BoolLit{Value: false, SpanInfo: e.Span()})
			if e.Op == "||" {
				thenExpr = &ast.BoolLit{Value: true, SpanInfo: e.Span()}
				elseExpr = e.Right
			}
			ifExpr := &ast.IfExpr{Cond: e.Left, Then: thenExpr, Else: elseExpr, SpanInfo: e.Span()}
			return c.compileIfExpr(ifExpr)
		}
		lhsType := normalizeTypeName(c.inferExprType(e.Left))
		rhsType := normalizeTypeName(c.inferExprType(e.Right))
		if (e.Op == "==" || e.Op == "!=") && lhsType != "" && lhsType == rhsType && c.isEnumTypeName(lhsType) {
			lhsOp := c.compileOperandBorrow(e.Left)
			rhsOp := c.compileOperandBorrow(e.Right)
			lhsTemp := c.operandToTemp(lhsOp, lhsType)
			rhsTemp := c.operandToTemp(rhsOp, lhsType)
			tagType := c.enumTagType[lhsType]
			if tagType == "" {
				tagType = "i32"
			}
			lhsTag := c.newTemp()
			c.setTempType(lhsTag, tagType)
			c.setTempBorrowed(lhsTag, false)
			c.emit(&ir.GetField{Dst: lhsTag, Src: lhsTemp, Field: "_tag"})
			rhsTag := c.newTemp()
			c.setTempType(rhsTag, tagType)
			c.setTempBorrowed(rhsTag, false)
			c.emit(&ir.GetField{Dst: rhsTag, Src: rhsTemp, Field: "_tag"})
			cmp := c.newTemp()
			c.setTempType(cmp, "bool")
			c.setTempBorrowed(cmp, false)
			c.emit(&ir.BinOp{Dst: cmp, Op: e.Op, Lhs: ir.TempOperand(lhsTag), Rhs: ir.TempOperand(rhsTag)})
			return cmp
		}
		lhs := c.compileOperandBorrow(e.Left)
		rhs := c.compileOperandBorrow(e.Right)
		t := c.newTemp()
		typ := c.inferExprType(e)
		c.setTempType(t, typ)
		c.setTempBorrowed(t, false)
		c.emit(&ir.BinOp{Dst: t, Op: e.Op, Lhs: lhs, Rhs: rhs})
		return t
	case *ast.CallExpr:
		args := make([]ir.Operand, 0, len(e.Args))
		for _, arg := range e.Args {
			args = append(args, c.compileCallArg(arg, e.Callee, len(args)))
		}
		if varInfo, ok := c.lookupVar(e.Callee); ok && varInfo.Closure {
			closureTemp := -1
			if varInfo.Temp >= 0 {
				closureTemp = varInfo.Temp
			} else {
				closureTemp = c.newTemp()
				c.setTempType(closureTemp, "Closure")
				c.emit(&ir.LoadVar{Dst: closureTemp, Name: varInfo.Name})
			}
			dst := c.newTemp()
			c.setTempType(dst, "unit")
			c.setTempBorrowed(dst, false)
			c.emit(&ir.CallClosure{Dst: dst, Closure: ir.TempOperand(closureTemp), Args: args})
			return dst
		}
		dst := c.newTemp()
		// Look up function return type
		if retType, ok := c.funcRetTypes[e.Callee]; ok {
			c.setTempType(dst, retType)
		} else {
			c.setTempType(dst, "unit") // Default return type
		}
		c.setTempBorrowed(dst, c.callResultBorrowed(e.Callee))
		c.emit(&ir.Call{Dst: dst, Callee: e.Callee, Args: args})
		return dst
	case *ast.MethodCallExpr:
		if e.EnumName != "" {
			return c.compileEnumVariant(e.EnumName, e.Method, e.Args, e.Span())
		}
		if e.ResolvedName == "" {
			c.diag.Add(e.Span(), "unresolved method call")
			return c.constZero()
		}
		args := make([]ir.Operand, 0, len(e.Args)+1)
		if e.ResolvedSelf {
			args = append(args, c.compileCallArg(e.Receiver, e.ResolvedName, 0))
		}
		for _, arg := range e.Args {
			args = append(args, c.compileCallArg(arg, e.ResolvedName, len(args)))
		}
		dst := c.newTemp()
		// Look up method return type
		if retType, ok := c.funcRetTypes[e.ResolvedName]; ok {
			c.setTempType(dst, retType)
		} else {
			c.setTempType(dst, "unit") // Default return type
		}
		c.setTempBorrowed(dst, c.callResultBorrowed(e.ResolvedName))
		c.emit(&ir.Call{Dst: dst, Callee: e.ResolvedName, Args: args})
		return dst
	case *ast.StructLit:
		fields := make([]ir.StructFieldInit, 0, len(e.Fields))
		for _, f := range e.Fields {
			src := c.compileOperandMove(f.Value)
			fields = append(fields, ir.StructFieldInit{Name: f.Name, Src: src})
		}
		dst := c.newTemp()
		c.setTempType(dst, e.Name)
		c.setTempBorrowed(dst, false)
		c.emit(&ir.MakeStruct{Dst: dst, Name: e.Name, Fields: fields})
		return dst
	case *ast.AccessExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			if _, ok := c.lookupVar(ident.Name); !ok {
				if enumDecl, ok := c.enums[ident.Name]; ok {
					if !enumHasVariant(enumDecl, e.Field) {
						c.diag.Add(e.Span(), fmt.Sprintf("unknown variant '%s'", e.Field))
						return c.constZero()
					}
					return c.compileEnumVariant(ident.Name, e.Field, nil, e.Span())
				}
			}
		}
		recv := c.compileExpr(e.Receiver)
		recvTypeRaw := c.inferExprType(e.Receiver)
		c.markTempBorrowedVar(recv)
		c.setTempBorrowed(recv, true)
		// Field access yields a borrowed view by default to avoid
		// double-drops when the base value remains owned elsewhere.
		borrowed := true
		dst := c.newTemp()
		fieldType := "i64"
		recvType := derefTypeName(recvTypeRaw)
		if recvType != "" {
			if decl, ok := c.structs[recvType]; ok {
				if fd := findField(decl, e.Field); fd != nil {
					fieldType = formatType(fd.Type)
				}
			} else if c.prog.TypeDecls != nil {
				if td, ok := c.prog.TypeDecls[recvType]; ok {
					for _, f := range td.Fields {
						if f.Name == e.Field {
							if f.Type != "" {
								fieldType = f.Type
							}
							break
						}
					}
				}
			}
		}
		c.setTempType(dst, fieldType)
		c.setTempBorrowed(dst, borrowed)
		c.emit(&ir.GetField{Dst: dst, Src: recv, Field: e.Field})
		return dst
	case *ast.IndexExpr:
		recv := c.compileOperandBorrow(e.Receiver)
		index := c.compileOperandBorrow(e.Index)
		if !recv.IsConst {
			c.setTempBorrowed(recv.Temp, true)
		}
		recvTypeRaw := c.inferExprType(e.Receiver)
		c.markTempBorrowedVar(recv.Temp)
		// Indexing borrows the element; moving out of containers is not
		// tracked precisely in stage0 and can lead to double-free.
		borrowed := true
		dst := c.newTemp()
		elemType := "i64"
		recvType := derefTypeName(recvTypeRaw)
		if isArrayTypeName(recvType) {
			if t := arrayElemTypeName(recvType); t != "" {
				elemType = t
			}
		}
		c.setTempType(dst, elemType)
		c.setTempBorrowed(dst, borrowed)
		c.emit(&ir.Index{Dst: dst, Array: recv, Index: index})
		if elemType == "String" {
			cloneTemp := c.newTempWithType("String")
			c.emit(&ir.Call{Dst: cloneTemp, Callee: "string_clone", Args: []ir.Operand{ir.TempOperand(dst)}})
			c.setTempBorrowed(cloneTemp, false)
			return cloneTemp
		}
		return dst
	case *ast.EnumVariantExpr:
		enumDecl, ok := c.enums[e.EnumName]
		if !ok {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown enum '%s'", e.EnumName))
			return c.constZero()
		}
		variant := enumVariant(enumDecl, e.Variant)
		if variant == nil {
			c.diag.Add(e.Span(), fmt.Sprintf("unknown variant '%s'", e.Variant))
			return c.constZero()
		}
		if e.Arg == nil && variant.Payload != nil {
			c.diag.Add(e.Span(), "missing payload for enum variant")
		}
		var args []ast.Expr
		if e.Arg != nil {
			args = []ast.Expr{e.Arg}
		}
		return c.compileEnumVariant(e.EnumName, e.Variant, args, e.Span())
	default:
		c.diag.Add(expr.Span(), "unsupported expression in stage 0")
		return c.constZero()
	}
}

func (c *Compiler) compileRefTarget(expr ast.Expr, mutable bool) int {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		varInfo, ok := c.lookupVar(e.Name)
		if !ok {
			if _, ok := c.consts[e.Name]; ok {
				c.diag.Add(e.Span(), fmt.Sprintf("cannot take reference to const '%s'", e.Name))
			} else {
				c.diag.Add(e.Span(), fmt.Sprintf("undefined variable '%s'", e.Name))
			}
			return c.constZero()
		}
		if varInfo.Moved && c.needsDropType(varInfo.Type) {
			c.diag.Add(e.Span(), fmt.Sprintf("use of moved value '%s'", e.Name))
			return c.constZero()
		}
		if varInfo.RefTemp >= 0 {
			return varInfo.RefTemp
		}
		if mutable && !varInfo.Mutable {
			c.diag.Add(e.Span(), fmt.Sprintf("cannot take &mut reference to immutable variable '%s'", e.Name))
			return c.constZero()
		}
		if isRefTypeName(varInfo.Type) {
			if varInfo.Temp >= 0 {
				return varInfo.Temp
			}
			t := c.newTemp()
			typ := varInfo.Type
			if typ == "" {
				typ = "i64"
			}
			c.setTempType(t, typ)
			c.emit(&ir.LoadVar{Dst: t, Name: varInfo.Name})
			return t
		}
		if varInfo.Type == "String" || varInfo.Type == "str" {
			if varInfo.Temp >= 0 {
				return varInfo.Temp
			}
			t := c.newTemp()
			c.setTempType(t, "str")
			c.emit(&ir.LoadVar{Dst: t, Name: varInfo.Name, Addr: false})
			return t
		}
		// Spill value parameters/lets to an addressable slot when taking a reference.
		if varInfo.Temp >= 0 {
			irName := varInfo.Name
			if irName == "" {
				count := c.nameCount[e.Name]
				c.nameCount[e.Name] = count + 1
				irName = e.Name
				if count > 0 {
					irName = fmt.Sprintf("%s#%d", e.Name, count)
				}
				c.emit(&ir.StoreVar{Name: irName, Src: ir.TempOperand(varInfo.Temp)})
				c.updateVar(e.Name, func(v *VarInfo) {
					v.Name = irName
					v.Temp = -1
				})
				varInfo.Name = irName
				varInfo.Temp = -1
			}
		}
		t := c.newTemp()
		typ := varInfo.Type
		if typ == "" {
			typ = "i64"
		}
		c.setTempType(t, "*"+typ)
		c.emit(&ir.LoadVar{Dst: t, Name: varInfo.Name, Addr: true})
		return t
	case *ast.AccessExpr:
		if !mutable {
			ft := c.inferExprType(expr)
			if ft == "String" || ft == "str" {
				return c.compileExpr(e)
			}
		}
		base := c.compileRefTarget(e.Receiver, mutable)
		c.setTempBorrowed(base, true)
		dst := c.newTemp()
		ft := c.inferExprType(expr)
		if ft == "" {
			ft = "i64"
		}
		c.setTempType(dst, "*"+ft)
		c.emit(&ir.FieldAddr{Dst: dst, Src: base, Field: e.Field})
		return dst
	case *ast.IndexExpr:
		if !mutable {
			elemType := ""
			recvType := derefTypeName(c.inferExprType(e.Receiver))
			if isArrayTypeName(recvType) {
				elemType = arrayElemTypeName(recvType)
			}
			if elemType == "String" || elemType == "str" {
				return c.compileExpr(e)
			}
		}
		base := c.compileRefTarget(e.Receiver, mutable)
		c.setTempBorrowed(base, true)
		index := c.compileOperandBorrow(e.Index)
		dst := c.newTemp()
		elemType := ""
		recvType := derefTypeName(c.inferExprType(e.Receiver))
		if isArrayTypeName(recvType) {
			elemType = arrayElemTypeName(recvType)
		}
		if elemType == "" {
			elemType = "i64"
		}
		c.setTempType(dst, "*"+elemType)
		c.emit(&ir.IndexAddr{Dst: dst, Base: base, Index: index})
		return dst
	case *ast.DerefExpr:
		// &*expr reuses the underlying reference
		return c.compileExpr(e.Expr)
	default:
		c.diag.Add(expr.Span(), "reference target must be identifier, field, or index in stage 0")
		return c.constZero()
	}
}

func (c *Compiler) compileQuoteExpr(e *ast.QuoteExpr) int {
	template, splices := c.quoteTemplate(e)
	templateOp := ir.StringOperand(template)
	spliceOp := c.compileQuoteSplices(splices)
	dst := c.newTemp()
	retType := quoteReturnType(e.Kind)
	c.setTempType(dst, retType)
	c.emit(&ir.Call{Dst: dst, Callee: quoteBuiltin(e.Kind), Args: []ir.Operand{templateOp, spliceOp}})
	return dst
}

func (c *Compiler) quoteTemplate(e *ast.QuoteExpr) (string, []ast.Expr) {
	var sb strings.Builder
	var splices []ast.Expr
	for _, part := range e.Parts {
		if part.Expr == nil {
			sb.WriteString(part.Text)
			continue
		}
		ph := fmt.Sprintf("__dast_splice_%d__", len(splices))
		sb.WriteString(ph)
		splices = append(splices, part.Expr)
	}
	return sb.String(), splices
}

func (c *Compiler) compileQuoteSplices(splices []ast.Expr) ir.Operand {
	dst := c.newTemp()
	c.setTempType(dst, "[Ast]")
	elems := make([]ir.Operand, 0, len(splices))
	for _, expr := range splices {
		elems = append(elems, c.compileOperandMove(expr))
	}
	c.emit(&ir.MakeArray{Dst: dst, Elems: elems})
	return ir.TempOperand(dst)
}

func quoteBuiltin(kind ast.QuoteKind) string {
	switch kind {
	case ast.QuoteExprKind:
		return "ast_expr"
	case ast.QuoteStmtKind:
		return "ast_stmt"
	case ast.QuoteItemKind:
		return "ast_item"
	case ast.QuoteBlockKind:
		return "ast_block"
	default:
		return "ast_expr"
	}
}

func quoteReturnType(kind ast.QuoteKind) string {
	switch kind {
	case ast.QuoteExprKind:
		return "AstExpr"
	case ast.QuoteStmtKind:
		return "AstStmt"
	case ast.QuoteItemKind:
		return "AstItem"
	case ast.QuoteBlockKind:
		return "AstBlock"
	default:
		return "AstExpr"
	}
}
