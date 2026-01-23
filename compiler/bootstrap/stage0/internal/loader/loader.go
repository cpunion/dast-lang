package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dastlang/internal/ast"
	"dastlang/internal/diag"
	"dastlang/internal/lexer"
	"dastlang/internal/parser"
	"dastlang/internal/source"
)

type LoadMode int

const (
	LoadNormal LoadMode = iota
	LoadTest
)

type importSpec struct {
	path  string
	alias string
	span  source.Span
}

type module struct {
	dir     string
	files   []string
	aliases map[string]struct{}
}

func normalizeImportPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return p
	}
	relPrefix := ""
	if strings.HasPrefix(p, "./") {
		relPrefix = "./"
		p = strings.TrimPrefix(p, "./")
	} else if strings.HasPrefix(p, "../") {
		relPrefix = "../"
		p = strings.TrimPrefix(p, "../")
	}
	if !strings.Contains(p, "/") && strings.Contains(p, ".") {
		p = strings.ReplaceAll(p, ".", "/")
	}
	return relPrefix + p
}

// LoadProgram loads a program with module imports resolved.
func LoadProgram(paths []string, mode LoadMode) (*ast.Program, *diag.Bag, error) {
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("missing input path")
	}
	includeTests := mode == LoadTest

	rootDir, entryDir, entryFiles, err := resolveEntry(paths, includeTests)
	if err != nil {
		return nil, nil, err
	}

	modules := map[string]*module{}
	visiting := map[string]bool{}
	var stack []string
	var scanDiags diag.Bag

	var loadModule func(dir string, files []string, include bool) error
	loadModule = func(dir string, files []string, include bool) error {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		if visiting[absDir] {
			stack = append(stack, absDir)
			return fmt.Errorf("circular import detected: %s", strings.Join(stack, " -> "))
		}
		if _, ok := modules[absDir]; ok {
			return nil
		}
		visiting[absDir] = true
		stack = append(stack, absDir)

		var modFiles []string
		if len(files) > 0 {
			modFiles = files
		} else {
			modFiles, err = listDastFiles(absDir, include)
			if err != nil {
				return err
			}
		}
		if len(modFiles) == 0 {
			return fmt.Errorf("no .dast files in %s", absDir)
		}

		aliases := map[string]struct{}{}
		for _, f := range modFiles {
			data, err := os.ReadFile(f)
			if err != nil {
				return fmt.Errorf("read failed: %s: %v", f, err)
			}
			imports, diags := scanImports(f, string(data))
			if diags != nil && len(diags.Items) > 0 {
				scanDiags.Items = append(scanDiags.Items, diags.Items...)
			}
			for _, imp := range imports {
				if imp.path == "" {
					continue
				}
				normPath := normalizeImportPath(imp.path)
				if imp.alias != "" {
					aliases[imp.alias] = struct{}{}
				} else {
					aliases[defaultAlias(normPath)] = struct{}{}
				}
				target, err := resolveImport(rootDir, absDir, normPath, imp.span)
				if err != nil {
					scanDiags.Add(imp.span, err.Error())
					continue
				}
				if err := loadModule(target, nil, false); err != nil {
					return err
				}
			}
		}

		modules[absDir] = &module{dir: absDir, files: modFiles, aliases: aliases}
		visiting[absDir] = false
		stack = stack[:len(stack)-1]
		return nil
	}

	if err := loadModule(entryDir, entryFiles, includeTests); err != nil {
		return nil, nil, err
	}

	if scanDiags.HasErrors() {
		return nil, &scanDiags, nil
	}

	// Merge files from all modules
	fileSet := map[string]struct{}{}
	for _, mod := range modules {
		for _, f := range mod.files {
			fileSet[f] = struct{}{}
		}
	}
	var files []string
	for f := range fileSet {
		files = append(files, f)
	}
	sort.Strings(files)

	sources := make([]parser.Source, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("read failed: %s: %v", f, err)
		}
		sources = append(sources, parser.Source{Filename: f, Input: string(data)})
	}
	prog, diags := parser.ParseFiles(sources)
	if diags != nil && len(diags.Items) > 0 {
		return nil, diags, nil
	}

	// Build file -> aliases map
	fileAliases := map[string]map[string]struct{}{}
	for _, mod := range modules {
		for _, f := range mod.files {
			fileAliases[f] = mod.aliases
		}
	}

	stripModulePrefixes(prog, fileAliases)
	stripImports(prog)
	return prog, diags, nil
}

func resolveEntry(paths []string, includeTests bool) (rootDir string, entryDir string, entryFiles []string, err error) {
	if len(paths) == 1 {
		p := paths[0]
		info, statErr := os.Stat(p)
		if statErr == nil && info.IsDir() {
			entryDir, err = filepath.Abs(p)
			if err != nil {
				return "", "", nil, err
			}
			pkgRoot, _ := findPackageRoot(entryDir)
			if pkgRoot != "" {
				rootDir = codeRoot(pkgRoot)
				if entryDir == pkgRoot {
					entryDir = rootDir
				}
				if !isWithin(entryDir, rootDir) {
					return "", "", nil, fmt.Errorf("entry dir outside package source root: %s", entryDir)
				}
			} else {
				rootDir = entryDir
			}
			entryFiles, err = listDastFiles(entryDir, includeTests)
			if err != nil {
				return "", "", nil, err
			}
			return rootDir, entryDir, entryFiles, nil
		}
	}

	// Treat all args as files
	for _, p := range paths {
		info, statErr := os.Stat(p)
		if statErr != nil {
			return "", "", nil, fmt.Errorf("read failed: %s: %v", p, statErr)
		}
		if info.IsDir() {
			return "", "", nil, fmt.Errorf("expected file, got dir: %s", p)
		}
	}

	firstDir, err := filepath.Abs(filepath.Dir(paths[0]))
	if err != nil {
		return "", "", nil, err
	}
	entryDir = firstDir
	pkgRoot, _ := findPackageRoot(entryDir)
	if pkgRoot != "" {
		rootDir = codeRoot(pkgRoot)
		if !isWithin(entryDir, rootDir) {
			return "", "", nil, fmt.Errorf("entry dir outside package source root: %s", entryDir)
		}
	} else {
		rootDir = entryDir
	}

	var files []string
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", "", nil, err
		}
		if filepath.Dir(abs) != entryDir {
			return "", "", nil, fmt.Errorf("all entry files must be in the same directory")
		}
		if !isWithin(entryDir, rootDir) {
			return "", "", nil, fmt.Errorf("entry dir outside package source root: %s", entryDir)
		}
		files = append(files, abs)
	}

	if includeTests {
		testFiles, err := listDastFiles(entryDir, true)
		if err != nil {
			return "", "", nil, err
		}
		for _, tf := range testFiles {
			if strings.HasSuffix(tf, "_test.dast") && !containsFile(files, tf) {
				files = append(files, tf)
			}
		}
	}

	sort.Strings(files)
	return rootDir, entryDir, files, nil
}

func findPackageRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(dir, "dast.toml")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", nil
}

func codeRoot(pkgRoot string) string {
	if pkgRoot == "" {
		return ""
	}
	srcDir := filepath.Join(pkgRoot, "src")
	if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
		return srcDir
	}
	return pkgRoot
}

func isWithin(path string, root string) bool {
	if root == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, "..")
}

func listDastFiles(dir string, includeTests bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".dast") {
			continue
		}
		if !includeTests && strings.HasSuffix(name, "_test.dast") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	return files, nil
}

func containsFile(files []string, path string) bool {
	for _, f := range files {
		if f == path {
			return true
		}
	}
	return false
}

func defaultAlias(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	p = strings.TrimSuffix(p, "/")
	base := filepath.Base(p)
	if base == "." || base == "/" {
		return ""
	}
	return base
}

func resolveImport(rootDir string, curDir string, path string, span source.Span) (string, error) {
	path = normalizeImportPath(path)
	if path == "" {
		return "", fmt.Errorf("empty import path")
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute import path not allowed")
	}
	var target string
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		target = filepath.Join(curDir, path)
	} else {
		target = filepath.Join(rootDir, path)
	}
	target = filepath.Clean(target)
	rootClean := filepath.Clean(rootDir)
	rel, err := filepath.Rel(rootClean, target)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("import escapes root: %s", path)
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("import not found: %s", path)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("import path is not a directory: %s", path)
	}
	return target, nil
}

func scanImports(filename string, input string) ([]importSpec, *diag.Bag) {
	lx := lexer.New(filename, input)
	var toks []lexer.Token
	for {
		tok := lx.Next()
		toks = append(toks, tok)
		if tok.Kind == lexer.TokenEOF {
			break
		}
	}

	var out []importSpec
	diags := &diag.Bag{}
	depth := 0
	for i := 0; i < len(toks); i++ {
		tok := toks[i]
		switch tok.Kind {
		case lexer.TokenLBrace, lexer.TokenLParen, lexer.TokenLBracket:
			depth++
		case lexer.TokenRBrace, lexer.TokenRParen, lexer.TokenRBracket:
			if depth > 0 {
				depth--
			}
		}
		if depth != 0 {
			continue
		}
		if tok.Kind != lexer.TokenImport {
			continue
		}
		if i+1 >= len(toks) {
			diags.Add(tok.Span, "expected import path")
			continue
		}
		pathTok := toks[i+1]
		if pathTok.Kind != lexer.TokenString {
			diags.Add(pathTok.Span, "expected import path")
			continue
		}
		alias := ""
		if i+2 < len(toks) && toks[i+2].Kind == lexer.TokenAs {
			if i+3 >= len(toks) {
				diags.Add(toks[i+2].Span, "expected import alias")
				continue
			}
			aliasTok := toks[i+3]
			if aliasTok.Kind != lexer.TokenIdent {
				diags.Add(aliasTok.Span, "expected import alias")
				continue
			}
			alias = aliasTok.Lexeme
			i += 3
		} else {
			i += 1
		}
		out = append(out, importSpec{path: pathTok.Lexeme, alias: alias, span: pathTok.Span})
	}

	if len(diags.Items) == 0 {
		return out, nil
	}
	return out, diags
}

func stripImports(prog *ast.Program) {
	if prog == nil {
		return
	}
	items := make([]ast.Item, 0, len(prog.Items))
	for _, item := range prog.Items {
		if _, ok := item.(*ast.ImportDecl); ok {
			continue
		}
		items = append(items, item)
	}
	prog.Items = items
}

func stripModulePrefixes(prog *ast.Program, fileAliases map[string]map[string]struct{}) {
	if prog == nil {
		return
	}
	for _, item := range prog.Items {
		span := item.Span()
		aliases := fileAliases[span.Start.Filename]
		if len(aliases) == 0 {
			continue
		}
		rewriteItem(item, aliases)
	}
}

func rewriteItem(item ast.Item, aliases map[string]struct{}) {
	switch v := item.(type) {
	case *ast.Function:
		for i := range v.Params {
			v.Params[i].Type = rewriteType(v.Params[i].Type, aliases)
		}
		if v.ReturnType != nil {
			rt := rewriteType(*v.ReturnType, aliases)
			v.ReturnType = &rt
		}
		v.Body = rewriteBlock(v.Body, aliases)
	case *ast.StructDecl:
		for i := range v.Fields {
			v.Fields[i].Type = rewriteType(v.Fields[i].Type, aliases)
		}
	case *ast.EnumDecl:
		for i := range v.Variants {
			if v.Variants[i].Payload != nil {
				t := rewriteType(*v.Variants[i].Payload, aliases)
				v.Variants[i].Payload = &t
			}
		}
	case *ast.ConstDecl:
		if v.Type != nil {
			t := rewriteType(*v.Type, aliases)
			v.Type = &t
		}
	case *ast.ImplDecl:
		v.TypeName = stripPrefix(v.TypeName, aliases)
		for i := range v.Methods {
			rewriteItem(v.Methods[i], aliases)
		}
	}
}

func rewriteBlock(b *ast.Block, aliases map[string]struct{}) *ast.Block {
	if b == nil {
		return b
	}
	for i := range b.Stmts {
		b.Stmts[i] = rewriteStmt(b.Stmts[i], aliases)
	}
	return b
}

func rewriteStmt(s ast.Stmt, aliases map[string]struct{}) ast.Stmt {
	switch v := s.(type) {
	case *ast.LetStmt:
		if v.Type != nil {
			t := rewriteType(*v.Type, aliases)
			v.Type = &t
		}
		v.Init = rewriteExpr(v.Init, aliases)
		return v
	case *ast.AssignStmt:
		v.Target = rewriteExpr(v.Target, aliases)
		v.Value = rewriteExpr(v.Value, aliases)
		return v
	case *ast.ExprStmt:
		v.Expr = rewriteExpr(v.Expr, aliases)
		return v
	case *ast.ReturnStmt:
		if v.Value != nil {
			v.Value = rewriteExpr(v.Value, aliases)
		}
		return v
	case *ast.IfStmt:
		v.Cond = rewriteExpr(v.Cond, aliases)
		v.Then = rewriteBlock(v.Then, aliases)
		v.Else = rewriteBlock(v.Else, aliases)
		return v
	case *ast.WhileStmt:
		v.Cond = rewriteExpr(v.Cond, aliases)
		v.Body = rewriteBlock(v.Body, aliases)
		return v
	case *ast.MatchStmt:
		v.Expr = rewriteExpr(v.Expr, aliases)
		for i := range v.Arms {
			v.Arms[i].Pattern = rewritePattern(v.Arms[i].Pattern, aliases)
			v.Arms[i].Body = rewriteBlock(v.Arms[i].Body, aliases)
		}
		return v
	case *ast.Block:
		return rewriteBlock(v, aliases)
	}
	return s
}

func rewritePattern(p ast.Pattern, aliases map[string]struct{}) ast.Pattern {
	switch v := p.(type) {
	case *ast.VariantPattern:
		v.EnumName = stripPrefix(v.EnumName, aliases)
		return v
	}
	return p
}

func rewriteType(t ast.Type, aliases map[string]struct{}) ast.Type {
	out := t
	if t.IsArray && t.Elem != nil {
		elem := rewriteType(*t.Elem, aliases)
		out.Elem = &elem
	}
	out.Name = stripPrefix(out.Name, aliases)
	return out
}

func rewriteExpr(e ast.Expr, aliases map[string]struct{}) ast.Expr {
	switch v := e.(type) {
	case *ast.IdentExpr:
		v.Name = stripPrefix(v.Name, aliases)
		return v
	case *ast.CallExpr:
		v.Callee = stripPrefix(v.Callee, aliases)
		for i := range v.Args {
			v.Args[i] = rewriteExpr(v.Args[i], aliases)
		}
		return v
	case *ast.MethodCallExpr:
		v.Receiver = rewriteExpr(v.Receiver, aliases)
		for i := range v.Args {
			v.Args[i] = rewriteExpr(v.Args[i], aliases)
		}
		segs, ok := pathSegments(v.Receiver)
		if ok && len(segs) >= 1 && hasAlias(aliases, segs[0]) {
			if len(segs) == 1 {
				return &ast.CallExpr{Callee: v.Method, Args: v.Args, SpanInfo: v.SpanInfo}
			}
			v.Receiver = buildAccessExpr(segs[1:], v.Receiver.Span())
		}
		return v
	case *ast.StructLit:
		v.Name = stripPrefix(v.Name, aliases)
		for i := range v.Fields {
			v.Fields[i].Value = rewriteExpr(v.Fields[i].Value, aliases)
		}
		return v
	case *ast.AccessExpr:
		v.Receiver = rewriteExpr(v.Receiver, aliases)
		segs, ok := pathSegments(v)
		if ok && len(segs) >= 2 && hasAlias(aliases, segs[0]) {
			if len(segs) == 2 {
				return &ast.IdentExpr{Name: segs[1], SpanInfo: v.SpanInfo}
			}
			return buildAccessExpr(segs[1:], v.SpanInfo)
		}
		return v
	case *ast.IndexExpr:
		v.Receiver = rewriteExpr(v.Receiver, aliases)
		v.Index = rewriteExpr(v.Index, aliases)
		return v
	case *ast.UnaryExpr:
		v.Expr = rewriteExpr(v.Expr, aliases)
		return v
	case *ast.BinaryExpr:
		v.Left = rewriteExpr(v.Left, aliases)
		v.Right = rewriteExpr(v.Right, aliases)
		return v
	case *ast.RefExpr:
		v.Expr = rewriteExpr(v.Expr, aliases)
		return v
	case *ast.DerefExpr:
		v.Expr = rewriteExpr(v.Expr, aliases)
		return v
	case *ast.ArrayLit:
		for i := range v.Elems {
			v.Elems[i] = rewriteExpr(v.Elems[i], aliases)
		}
		return v
	case *ast.EnumVariantExpr:
		v.EnumName = stripPrefix(v.EnumName, aliases)
		if v.Arg != nil {
			v.Arg = rewriteExpr(v.Arg, aliases)
		}
		return v
	}
	return e
}

func stripPrefix(name string, aliases map[string]struct{}) string {
	if name == "" {
		return name
	}
	parts := strings.SplitN(name, ".", 2)
	if len(parts) != 2 {
		return name
	}
	if hasAlias(aliases, parts[0]) {
		return parts[1]
	}
	return name
}

func hasAlias(aliases map[string]struct{}, name string) bool {
	if len(aliases) == 0 || name == "" {
		return false
	}
	_, ok := aliases[name]
	return ok
}

func pathSegments(e ast.Expr) ([]string, bool) {
	switch v := e.(type) {
	case *ast.IdentExpr:
		return []string{v.Name}, true
	case *ast.AccessExpr:
		segs, ok := pathSegments(v.Receiver)
		if !ok {
			return nil, false
		}
		return append(segs, v.Field), true
	}
	return nil, false
}

func buildAccessExpr(parts []string, span source.Span) ast.Expr {
	if len(parts) == 0 {
		return &ast.IdentExpr{Name: "", SpanInfo: span}
	}
	var expr ast.Expr = &ast.IdentExpr{Name: parts[0], SpanInfo: span}
	for i := 1; i < len(parts); i++ {
		expr = &ast.AccessExpr{Receiver: expr, Field: parts[i], SpanInfo: span}
	}
	return expr
}
