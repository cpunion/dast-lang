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

type LoadOptions struct {
	AllowMultiDir bool
}

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

type depRef struct {
	name     string
	pkgRoot  string
	codeRoot string
}

type pkgInfo struct {
	pkgRoot      string
	codeRoot     string
	deps         map[string]depRef
	includeTests bool
	hasPackage   bool
	workspace    *workspaceInfo
}

type depSpec struct {
	name      string
	path      string
	line      int
	workspace bool
}

type parseError struct {
	line int
	msg  string
}

type workspaceInfo struct {
	root    string
	members []string
	deps    map[string]depSpec
}

var stdlibRootCached string
var stdlibRootInit bool

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
		if !strings.HasPrefix(p, ".") && !strings.HasSuffix(p, ".dast") {
			p = strings.ReplaceAll(p, ".", "/")
		}
	}
	return relPrefix + p
}

func stdlibRoot() string {
	if stdlibRootInit {
		return stdlibRootCached
	}
	stdlibRootInit = true
	if v := os.Getenv("DAST_STDLIB"); v != "" {
		stdlibRootCached = v
		return v
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	root := findGoModRoot(filepath.Dir(exe))
	if root == "" {
		return ""
	}
	cand := filepath.Join(root, "stdlib")
	if info, err := os.Stat(cand); err == nil && info.IsDir() {
		stdlibRootCached = cand
		return cand
	}
	return ""
}

func findGoModRoot(dir string) string {
	cur := dir
	for {
		path := filepath.Join(cur, "go.mod")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return ""
}

// LoadProgram loads a program with module imports resolved.
func LoadProgram(paths []string, mode LoadMode, opts LoadOptions) (*ast.Program, *diag.Bag, error) {
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("missing input path")
	}
	includeTests := mode == LoadTest
	packages := map[string]*pkgInfo{}
	loadingPkgs := map[string]bool{}
	workspaces := map[string]*workspaceInfo{}

	var scanDiags diag.Bag
	entryPkg, entryDir, entryFiles, err := resolveEntry(paths, includeTests, packages, loadingPkgs, workspaces, &scanDiags, opts)
	if err != nil {
		return nil, nil, err
	}
	if scanDiags.HasErrors() {
		return nil, &scanDiags, nil
	}

	modules := map[string]*module{}
	visiting := map[string]bool{}
	var stack []string

	var loadModule func(pkg *pkgInfo, dir string, files []string, include bool) error
	loadModule = func(pkg *pkgInfo, dir string, files []string, include bool) error {
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
				target, targetPkg, err := resolveImport(pkg, absDir, normPath, imp.span, packages, loadingPkgs, workspaces, &scanDiags)
				if err != nil {
					scanDiags.Add(imp.span, err.Error())
					continue
				}
				if err := loadModule(targetPkg, target, nil, false); err != nil {
					return err
				}
			}
		}

		modules[absDir] = &module{dir: absDir, files: modFiles, aliases: aliases}
		visiting[absDir] = false
		stack = stack[:len(stack)-1]
		return nil
	}

	if err := loadModule(entryPkg, entryDir, entryFiles, includeTests); err != nil {
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

func resolveEntry(paths []string, includeTests bool, packages map[string]*pkgInfo, loading map[string]bool, workspaces map[string]*workspaceInfo, diags *diag.Bag, opts LoadOptions) (entryPkg *pkgInfo, entryDir string, entryFiles []string, err error) {
	if len(paths) == 1 {
		p := paths[0]
		info, statErr := os.Stat(p)
		if statErr == nil && info.IsDir() {
			entryDir, err = filepath.Abs(p)
			if err != nil {
				return nil, "", nil, err
			}
			pkgRoot, _ := findPackageRoot(entryDir)
			if pkgRoot != "" {
				entryPkg = loadPackage(pkgRoot, includeTests, packages, loading, workspaces, diags)
				if entryPkg == nil {
					return nil, "", nil, fmt.Errorf("failed to load package: %s", pkgRoot)
				}
				codeRoot := entryPkg.codeRoot
				if entryDir == pkgRoot {
					entryDir = codeRoot
				}
				if !isWithin(entryDir, codeRoot) {
					return nil, "", nil, fmt.Errorf("entry dir outside package source root: %s", entryDir)
				}
			} else {
				entryPkg = loadPackage(entryDir, includeTests, packages, loading, workspaces, diags)
				if entryPkg == nil {
					return nil, "", nil, fmt.Errorf("failed to load package: %s", entryDir)
				}
			}
			entryFiles, err = listDastFiles(entryDir, includeTests)
			if err != nil {
				return nil, "", nil, err
			}
			return entryPkg, entryDir, entryFiles, nil
		}
	}

	// Treat remaining args as files or directories
	var files []string
	var dirs []string
	fileSet := map[string]struct{}{}
	for _, p := range paths {
		info, statErr := os.Stat(p)
		if statErr != nil {
			return nil, "", nil, fmt.Errorf("read failed: %s: %v", p, statErr)
		}
		if info.IsDir() {
			absDir, err := filepath.Abs(p)
			if err != nil {
				return nil, "", nil, err
			}
			dirs = append(dirs, absDir)
			modFiles, err := listDastFiles(absDir, includeTests)
			if err != nil {
				return nil, "", nil, err
			}
			if len(modFiles) == 0 {
				return nil, "", nil, fmt.Errorf("no .dast files in %s", absDir)
			}
			for _, f := range modFiles {
				fileSet[f] = struct{}{}
			}
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, "", nil, err
		}
		dirs = append(dirs, filepath.Dir(abs))
		fileSet[abs] = struct{}{}
	}

	if len(dirs) == 0 {
		return nil, "", nil, fmt.Errorf("missing input path")
	}

	entryDir = dirs[0]
	if opts.AllowMultiDir {
		entryDir = commonDir(dirs)
		entryPkg = loadPackage(entryDir, includeTests, packages, loading, workspaces, diags)
		if entryPkg == nil {
			return nil, "", nil, fmt.Errorf("failed to load package: %s", entryDir)
		}
	} else {
		pkgRoot, _ := findPackageRoot(entryDir)
		if pkgRoot != "" {
			entryPkg = loadPackage(pkgRoot, includeTests, packages, loading, workspaces, diags)
			if entryPkg == nil {
				return nil, "", nil, fmt.Errorf("failed to load package: %s", pkgRoot)
			}
			if !isWithin(entryDir, entryPkg.codeRoot) {
				return nil, "", nil, fmt.Errorf("entry dir outside package source root: %s", entryDir)
			}
		} else {
			entryPkg = loadPackage(entryDir, includeTests, packages, loading, workspaces, diags)
			if entryPkg == nil {
				return nil, "", nil, fmt.Errorf("failed to load package: %s", entryDir)
			}
		}
	}

	for f := range fileSet {
		if !opts.AllowMultiDir && filepath.Dir(f) != entryDir {
			return nil, "", nil, fmt.Errorf("all entry files must be in the same directory")
		}
		if !opts.AllowMultiDir && !isWithin(entryDir, entryPkg.codeRoot) {
			return nil, "", nil, fmt.Errorf("entry dir outside package source root: %s", entryDir)
		}
		files = append(files, f)
	}

	sort.Strings(files)
	return entryPkg, entryDir, files, nil
}

func commonDir(dirs []string) string {
	if len(dirs) == 0 {
		return ""
	}
	common := dirs[0]
	for _, d := range dirs[1:] {
		for !isWithin(d, common) {
			parent := filepath.Dir(common)
			if parent == common {
				return common
			}
			common = parent
		}
	}
	return common
}

type manifestInfo struct {
	hasPackage    bool
	hasWorkspace  bool
	members       []string
	workspaceDeps []depSpec
}

func parseManifestInfo(manifestPath string) (manifestInfo, []parseError) {
	info := manifestInfo{}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return info, []parseError{{line: 1, msg: "failed to read manifest"}}
	}
	lines := strings.Split(string(data), "\n")
	section := ""
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			switch section {
			case "package":
				info.hasPackage = true
			case "workspace":
				info.hasWorkspace = true
			}
			continue
		}
		switch section {
		case "workspace":
			if strings.HasPrefix(line, "members") {
				eq := strings.Index(line, "=")
				if eq < 0 {
					continue
				}
				val := strings.TrimSpace(line[eq+1:])
				members, err := parseStringArray(val)
				if err != "" {
					return info, []parseError{{line: lineNo, msg: err}}
				}
				info.members = members
			}
		case "workspace.dependencies":
			eq := strings.Index(line, "=")
			if eq < 0 {
				continue
			}
			name := strings.TrimSpace(line[:eq])
			value := strings.TrimSpace(line[eq+1:])
			if name == "" {
				continue
			}
			pathVal, workspace, perr := parsePathDep(value)
			if perr != "" {
				return info, []parseError{{line: lineNo, msg: perr}}
			}
			if workspace {
				return info, []parseError{{line: lineNo, msg: "workspace dependencies must use path"}}
			}
			info.workspaceDeps = append(info.workspaceDeps, depSpec{name: name, path: pathVal, line: lineNo})
		}
	}
	return info, nil
}

func parseStringArray(value string) ([]string, string) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, "expected array literal"
	}
	inner := strings.TrimSpace(value[1 : len(value)-1])
	if inner == "" {
		return []string{}, ""
	}
	parts := strings.Split(inner, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		str, ok := parseStringLiteral(part)
		if !ok {
			return nil, "array entries must be strings"
		}
		out = append(out, str)
	}
	return out, ""
}

func findWorkspaceRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(dir, "dast.toml")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			mInfo, _ := parseManifestInfo(path)
			if mInfo.hasWorkspace {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", nil
}

func loadWorkspace(root string, workspaces map[string]*workspaceInfo, diags *diag.Bag) *workspaceInfo {
	if root == "" {
		return nil
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	if ws, ok := workspaces[absRoot]; ok {
		return ws
	}
	manifestPath := filepath.Join(absRoot, "dast.toml")
	info, errs := parseManifestInfo(manifestPath)
	if !info.hasWorkspace {
		return nil
	}
	for _, e := range errs {
		if diags != nil {
			addManifestError(diags, manifestPath, e.line, e.msg)
		}
	}
	depMap := map[string]depSpec{}
	for _, dep := range info.workspaceDeps {
		if dep.name == "" {
			continue
		}
		depMap[dep.name] = dep
	}
	ws := &workspaceInfo{
		root:    absRoot,
		members: info.members,
		deps:    depMap,
	}
	workspaces[absRoot] = ws
	return ws
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

func codeRoot(pkgRoot string, hasPackage bool) string {
	if pkgRoot == "" {
		return ""
	}
	if !hasPackage {
		return pkgRoot
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

func resolveWithin(baseDir string, rootDir string, relPath string) (string, error) {
	if relPath == "" {
		relPath = "."
	}
	target := filepath.Join(baseDir, relPath)
	target = filepath.Clean(target)
	rootClean := filepath.Clean(rootDir)
	rel, err := filepath.Rel(rootClean, target)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("import escapes root: %s", relPath)
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("import not found: %s", relPath)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("import path is not a directory: %s", relPath)
	}
	return target, nil
}

func loadPackage(pkgRoot string, includeTests bool, packages map[string]*pkgInfo, loading map[string]bool, workspaces map[string]*workspaceInfo, diags *diag.Bag) *pkgInfo {
	if pkgRoot == "" {
		return nil
	}
	absRoot, err := filepath.Abs(pkgRoot)
	if err != nil {
		return nil
	}
	if pkg, ok := packages[absRoot]; ok {
		return pkg
	}
	if loading[absRoot] {
		if diags != nil {
			addManifestError(diags, filepath.Join(absRoot, "dast.toml"), 1, "circular dependency detected")
		}
		return nil
	}
	loading[absRoot] = true

	info := manifestInfo{}
	manifestPath := filepath.Join(absRoot, "dast.toml")
	if _, err := os.Stat(manifestPath); err == nil {
		var errs []parseError
		info, errs = parseManifestInfo(manifestPath)
		for _, e := range errs {
			if diags != nil {
				addManifestError(diags, manifestPath, e.line, e.msg)
			}
		}
	} else {
		info.hasPackage = true
	}

	code := codeRoot(absRoot, info.hasPackage)
	pkg := &pkgInfo{
		pkgRoot:      absRoot,
		codeRoot:     code,
		deps:         map[string]depRef{},
		includeTests: includeTests,
		hasPackage:   info.hasPackage,
	}
	packages[absRoot] = pkg
	wsRoot, _ := findWorkspaceRoot(absRoot)
	if wsRoot != "" {
		pkg.workspace = loadWorkspace(wsRoot, workspaces, diags)
	}

	if _, err := os.Stat(manifestPath); err == nil {
		deps, devDeps, buildDeps, errs := parseManifestDeps(manifestPath)
		for _, e := range errs {
			if diags != nil {
				addManifestError(diags, manifestPath, e.line, e.msg)
			}
		}
		specs := deps
		if includeTests {
			specs = append(specs, devDeps...)
		}
		specs = append(specs, buildDeps...)
		for _, spec := range specs {
			if spec.name == "" {
				continue
			}
			if _, exists := pkg.deps[spec.name]; exists {
				if diags != nil {
					addManifestError(diags, manifestPath, spec.line, fmt.Sprintf("duplicate dependency '%s'", spec.name))
				}
				continue
			}
			baseRoot := absRoot
			if spec.workspace {
				if pkg.workspace == nil {
					if diags != nil {
						addManifestError(diags, manifestPath, spec.line, fmt.Sprintf("dependency '%s' requires workspace", spec.name))
					}
					continue
				}
				wsSpec, ok := pkg.workspace.deps[spec.name]
				if !ok || wsSpec.path == "" {
					if diags != nil {
						addManifestError(diags, manifestPath, spec.line, fmt.Sprintf("workspace dependency '%s' not found", spec.name))
					}
					continue
				}
				spec.path = wsSpec.path
				baseRoot = pkg.workspace.root
			}
			if spec.path == "" {
				if diags != nil {
					addManifestError(diags, manifestPath, spec.line, fmt.Sprintf("dependency '%s' missing path", spec.name))
				}
				continue
			}
			if filepath.IsAbs(spec.path) {
				if diags != nil {
					addManifestError(diags, manifestPath, spec.line, fmt.Sprintf("dependency '%s' path must be relative", spec.name))
				}
				continue
			}
			depRoot := filepath.Clean(filepath.Join(baseRoot, spec.path))
			info, err := os.Stat(depRoot)
			if err != nil {
				if diags != nil {
					addManifestError(diags, manifestPath, spec.line, fmt.Sprintf("dependency '%s' not found: %s", spec.name, spec.path))
				}
				continue
			}
			if !info.IsDir() {
				if diags != nil {
					addManifestError(diags, manifestPath, spec.line, fmt.Sprintf("dependency '%s' path is not a directory: %s", spec.name, spec.path))
				}
				continue
			}
			depPkg := loadPackage(depRoot, includeTests, packages, loading, workspaces, diags)
			if depPkg == nil {
				if diags != nil {
					addManifestError(diags, manifestPath, spec.line, fmt.Sprintf("failed to load dependency '%s'", spec.name))
				}
				continue
			}
			pkg.deps[spec.name] = depRef{name: spec.name, pkgRoot: depPkg.pkgRoot, codeRoot: depPkg.codeRoot}
		}
	}

	loading[absRoot] = false
	return pkg
}

func parseManifestDeps(manifestPath string) (deps []depSpec, devDeps []depSpec, buildDeps []depSpec, errs []parseError) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, nil, nil, []parseError{{line: 1, msg: "failed to read manifest"}}
	}
	lines := strings.Split(string(data), "\n")
	section := ""
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		if section != "dependencies" && section != "dev-dependencies" && section != "build-dependencies" {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		name := strings.TrimSpace(line[:eq])
		value := strings.TrimSpace(line[eq+1:])
		if name == "" {
			continue
		}
		pathVal, workspace, perr := parsePathDep(value)
		if perr != "" {
			errs = append(errs, parseError{line: lineNo, msg: perr})
			continue
		}
		spec := depSpec{name: name, path: pathVal, line: lineNo, workspace: workspace}
		switch section {
		case "dependencies":
			deps = append(deps, spec)
		case "dev-dependencies":
			devDeps = append(devDeps, spec)
		case "build-dependencies":
			buildDeps = append(buildDeps, spec)
		}
	}
	return deps, devDeps, buildDeps, errs
}

func parsePathDep(value string) (string, bool, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false, "dependency must be inline table with path"
	}
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		if inner == "" {
			return "", false, "dependency must specify path or workspace"
		}
		parts := strings.Split(inner, ",")
		pathVal := ""
		workspace := false
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				return "", false, "dependency entry must be key = value"
			}
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			if key == "path" {
				str, ok := parseStringLiteral(val)
				if !ok {
					return "", false, "dependency path must be string"
				}
				pathVal = str
				continue
			} else {
				if key == "workspace" {
					if val == "true" {
						workspace = true
						continue
					}
					return "", false, "workspace must be true"
				}
				return "", false, "only path or workspace dependencies are supported"
			}
		}
		if workspace && pathVal != "" {
			return "", false, "dependency cannot specify both path and workspace"
		}
		if workspace {
			return "", true, ""
		}
		if pathVal == "" {
			return "", false, "dependency must specify path"
		}
		return pathVal, false, ""
	}
	return "", false, "only path dependencies are supported"
}

func parseStringLiteral(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", false
	}
	return value[1 : len(value)-1], true
}

func addManifestError(diags *diag.Bag, file string, line int, msg string) {
	if diags == nil {
		return
	}
	if line <= 0 {
		line = 1
	}
	span := source.Span{
		Start: source.Position{Filename: file, Line: line, Column: 1},
		End:   source.Position{Filename: file, Line: line, Column: 1},
	}
	diags.Add(span, msg)
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

func resolveImport(pkg *pkgInfo, curDir string, path string, span source.Span, packages map[string]*pkgInfo, loading map[string]bool, workspaces map[string]*workspaceInfo, diags *diag.Bag) (string, *pkgInfo, error) {
	path = normalizeImportPath(path)
	if path == "" {
		return "", nil, fmt.Errorf("empty import path")
	}
	if filepath.IsAbs(path) {
		return "", nil, fmt.Errorf("absolute import path not allowed")
	}
	if pkg == nil {
		return "", nil, fmt.Errorf("missing package context")
	}
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		target, err := resolveWithin(curDir, pkg.codeRoot, path)
		return target, pkg, err
	}
	if path == "std" || strings.HasPrefix(path, "std/") {
		stdRoot := stdlibRoot()
		if stdRoot == "" {
			return "", nil, fmt.Errorf("stdlib not found for import: %s", path)
		}
		rel := strings.TrimPrefix(path, "std/")
		target, err := resolveWithin(stdRoot, stdRoot, rel)
		return target, pkg, err
	}
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		if dep, ok := pkg.deps[parts[0]]; ok {
			depPkg := loadPackage(dep.pkgRoot, pkg.includeTests, packages, loading, workspaces, diags)
			if depPkg == nil {
				return "", nil, fmt.Errorf("failed to load dependency '%s'", dep.name)
			}
			base := depPkg.codeRoot
			rel := ""
			if len(parts) > 1 {
				rel = filepath.Join(parts[1:]...)
			}
			target, err := resolveWithin(base, depPkg.codeRoot, rel)
			return target, depPkg, err
		}
	}
	target, err := resolveWithin(pkg.codeRoot, pkg.codeRoot, path)
	return target, pkg, err
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
	case *ast.CompileItem:
		v.Expr = rewriteExpr(v.Expr, aliases)
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
	case *ast.LetPatternStmt:
		v.Pattern = rewritePattern(v.Pattern, aliases)
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
	case *ast.IfLetStmt:
		v.Pattern = rewritePattern(v.Pattern, aliases)
		v.Expr = rewriteExpr(v.Expr, aliases)
		v.Then = rewriteBlock(v.Then, aliases)
		v.Else = rewriteBlock(v.Else, aliases)
		return v
	case *ast.WhileStmt:
		v.Cond = rewriteExpr(v.Cond, aliases)
		v.Body = rewriteBlock(v.Body, aliases)
		return v
	case *ast.WhileLetStmt:
		v.Pattern = rewritePattern(v.Pattern, aliases)
		v.Expr = rewriteExpr(v.Expr, aliases)
		v.Body = rewriteBlock(v.Body, aliases)
		return v
	case *ast.LoopStmt:
		v.Body = rewriteBlock(v.Body, aliases)
		return v
	case *ast.BreakStmt:
		if v.Value != nil {
			v.Value = rewriteExpr(v.Value, aliases)
		}
		return v
	case *ast.ContinueStmt:
		return v
	case *ast.ForStmt:
		v.Pattern = rewritePattern(v.Pattern, aliases)
		v.Expr = rewriteExpr(v.Expr, aliases)
		v.Body = rewriteBlock(v.Body, aliases)
		return v
	case *ast.MatchStmt:
		v.Expr = rewriteExpr(v.Expr, aliases)
		for i := range v.Arms {
			v.Arms[i].Pattern = rewritePattern(v.Arms[i].Pattern, aliases)
			if v.Arms[i].Guard != nil {
				v.Arms[i].Guard = rewriteExpr(v.Arms[i].Guard, aliases)
			}
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
	case *ast.BindingPattern:
		return v
	case *ast.VariantPattern:
		v.EnumName = stripPrefix(v.EnumName, aliases)
		return v
	case *ast.StructPattern:
		v.StructName = stripPrefix(v.StructName, aliases)
		for i := range v.Fields {
			if v.Fields[i].Pattern != nil {
				v.Fields[i].Pattern = rewritePattern(v.Fields[i].Pattern, aliases)
			}
		}
		return v
	case *ast.OrPattern:
		for i := range v.Alts {
			v.Alts[i] = rewritePattern(v.Alts[i], aliases)
		}
		return v
	case *ast.TuplePattern:
		for i := range v.Elems {
			v.Elems[i] = rewritePattern(v.Elems[i], aliases)
		}
		return v
	case *ast.ArrayPattern:
		for i := range v.Elems {
			v.Elems[i] = rewritePattern(v.Elems[i], aliases)
		}
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
	if t.IsTuple && len(t.TupleElems) > 0 {
		elems := make([]ast.Type, 0, len(t.TupleElems))
		for _, e := range t.TupleElems {
			elems = append(elems, rewriteType(e, aliases))
		}
		out.TupleElems = elems
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
	case *ast.CompileExpr:
		v.Expr = rewriteExpr(v.Expr, aliases)
		return v
	case *ast.MacroCallExpr:
		v.Callee = rewriteExpr(v.Callee, aliases)
		for i := range v.Args {
			v.Args[i] = rewriteExpr(v.Args[i], aliases)
		}
		return v
	case *ast.QuoteExpr:
		for i := range v.Parts {
			if v.Parts[i].Expr != nil {
				v.Parts[i].Expr = rewriteExpr(v.Parts[i].Expr, aliases)
			}
		}
		return v
	case *ast.ArrayLit:
		for i := range v.Elems {
			v.Elems[i] = rewriteExpr(v.Elems[i], aliases)
		}
		return v
	case *ast.TupleLit:
		for i := range v.Elems {
			v.Elems[i] = rewriteExpr(v.Elems[i], aliases)
		}
		return v
	case *ast.BlockExpr:
		v.Block = rewriteBlock(v.Block, aliases)
		return v
	case *ast.LoopExpr:
		v.Body = rewriteBlock(v.Body, aliases)
		return v
	case *ast.IfExpr:
		v.Cond = rewriteExpr(v.Cond, aliases)
		v.Then = rewriteExpr(v.Then, aliases)
		if v.Else != nil {
			v.Else = rewriteExpr(v.Else, aliases)
		}
		return v
	case *ast.MatchExpr:
		v.Expr = rewriteExpr(v.Expr, aliases)
		for i := range v.Arms {
			v.Arms[i].Pattern = rewritePattern(v.Arms[i].Pattern, aliases)
			if v.Arms[i].Guard != nil {
				v.Arms[i].Guard = rewriteExpr(v.Arms[i].Guard, aliases)
			}
			v.Arms[i].Body = rewriteBlock(v.Arms[i].Body, aliases)
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
