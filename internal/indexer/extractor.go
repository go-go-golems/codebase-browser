package indexer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/tools/go/packages"
)

// ExtractOptions controls how Extract loads and walks packages.
type ExtractOptions struct {
	ModuleRoot string
	Patterns   []string
	// IncludeTests loads *_test.go packages. Defaults true.
	IncludeTests bool
}

// Extract loads Go packages under ModuleRoot matching Patterns and builds an
// Index. See design §7.
func Extract(opts ExtractOptions) (*Index, error) {
	if opts.ModuleRoot == "" {
		opts.ModuleRoot = "."
	}
	if len(opts.Patterns) == 0 {
		opts.Patterns = []string{"./..."}
	}
	absRoot, err := filepath.Abs(opts.ModuleRoot)
	if err != nil {
		return nil, fmt.Errorf("abs module root: %w", err)
	}

	// GOWORK=off disables Go workspace mode so that packages.Load resolves
	// patterns against the local go.mod even when a parent go.work exists.
	// This is essential when extracting from git worktrees created inside
	// the repo directory — they inherit the parent go.work but aren't listed
	// as workspace modules, causing packages.Load to fail silently.
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedImports | packages.NeedModule,
		Dir:   absRoot,
		Fset:  token.NewFileSet(),
		Tests: opts.IncludeTests,
		Env:   append(os.Environ(), "GOWORK=off"),
	}

	pkgs, err := packages.Load(cfg, opts.Patterns...)
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %w", err)
	}
	// Non-fatal: we tolerate packages with errors to still produce a useful index.

	idx := &Index{
		Version:     "1",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		GoVersion:   runtime.Version(),
	}
	if len(pkgs) > 0 && pkgs[0].Module != nil {
		idx.Module = pkgs[0].Module.Path
	}

	seenPkg := map[string]bool{}
	seenFile := map[string]bool{}

	for _, p := range pkgs {
		if p.PkgPath == "" {
			continue
		}
		if seenPkg[p.PkgPath] {
			continue
		}
		seenPkg[p.PkgPath] = true

		pkg := Package{
			ID:         PackageID(p.PkgPath),
			ImportPath: p.PkgPath,
			Name:       p.Name,
		}

		// Package-level doc comment: grab from the first file with a Doc.
		for _, f := range p.Syntax {
			if f.Doc != nil {
				pkg.Doc = strings.TrimSpace(f.Doc.Text())
				break
			}
		}

		// Files
		for fi, f := range p.Syntax {
			if fi >= len(p.CompiledGoFiles) {
				break
			}
			absPath := p.CompiledGoFiles[fi]
			rel, err := filepath.Rel(absRoot, absPath)
			if err != nil || strings.HasPrefix(rel, "..") {
				continue
			}
			rel = filepath.ToSlash(rel)

			if seenFile[rel] {
				continue
			}
			seenFile[rel] = true

			data, err := os.ReadFile(absPath)
			if err != nil {
				continue
			}
			sum := sha256.Sum256(data)
			lineCount := 1
			for _, b := range data {
				if b == '\n' {
					lineCount++
				}
			}

			file := File{
				ID:        FileID(rel),
				Path:      rel,
				PackageID: pkg.ID,
				Size:      len(data),
				LineCount: lineCount,
				SHA256:    hex.EncodeToString(sum[:]),
				BuildTags: parseBuildTags(f),
			}
			idx.Files = append(idx.Files, file)
			pkg.FileIDs = append(pkg.FileIDs, file.ID)

			// Walk top-level decls in this file only.
			for _, decl := range f.Decls {
				syms := extractDecl(p, cfg.Fset, decl, rel, pkg.ID, file.ID)
				for _, s := range syms {
					idx.Symbols = append(idx.Symbols, s)
					pkg.SymbolIDs = append(pkg.SymbolIDs, s.ID)
				}
			}

			// Walk function bodies for cross-references.
			idx.addRefsForFile(p, f, cfg.Fset, file.ID)
		}

		idx.Packages = append(idx.Packages, pkg)
	}

	stampLanguage(idx, "go")
	sortIndex(idx)
	return idx, nil
}

// stampLanguage sets Language on every Package/File/Symbol in the index.
// Cheaper than threading a field through every constructor and matches the
// two-pass approach in sortIndex/deduplicate.
func stampLanguage(idx *Index, lang string) {
	for i := range idx.Packages {
		idx.Packages[i].Language = lang
	}
	for i := range idx.Files {
		idx.Files[i].Language = lang
	}
	for i := range idx.Symbols {
		idx.Symbols[i].Language = lang
	}
}

func parseBuildTags(f *ast.File) []string {
	var tags []string
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			t := strings.TrimSpace(c.Text)
			if strings.HasPrefix(t, "//go:build ") {
				tags = append(tags, strings.TrimSpace(strings.TrimPrefix(t, "//go:build")))
			}
		}
	}
	return tags
}

func extractDecl(p *packages.Package, fset *token.FileSet, decl ast.Decl, relPath, pkgID, fileID string) []Symbol {
	var out []Symbol
	switch d := decl.(type) {
	case *ast.FuncDecl:
		s := funcSymbol(p, fset, d, pkgID, fileID)
		out = append(out, s)
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch sp := spec.(type) {
			case *ast.TypeSpec:
				out = append(out, typeSymbol(p, fset, d, sp, pkgID, fileID))
			case *ast.ValueSpec:
				kind := "var"
				if d.Tok == token.CONST {
					kind = "const"
				}
				for _, name := range sp.Names {
					out = append(out, valueSymbol(p, fset, d, sp, name, kind, pkgID, fileID))
				}
			}
		}
	}
	_ = relPath
	return out
}

func funcSymbol(p *packages.Package, fset *token.FileSet, fn *ast.FuncDecl, pkgID, fileID string) Symbol {
	kind := "func"
	var recv *Receiver
	name := fn.Name.Name

	var id string
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		kind = "method"
		recvType, ptr := recvTypeName(fn.Recv.List[0].Type)
		recv = &Receiver{TypeName: recvType, Pointer: ptr}
		id = MethodID(p.PkgPath, recvType, name)
	} else {
		id = SymbolID(p.PkgPath, kind, name, "")
	}

	sig := printNode(fset, &ast.FuncDecl{
		Name: fn.Name,
		Type: fn.Type,
		Recv: fn.Recv,
	})

	return Symbol{
		ID:         id,
		Kind:       kind,
		Name:       name,
		PackageID:  pkgID,
		FileID:     fileID,
		Range:      rangeOf(fset, fn),
		Doc:        docText(fn.Doc),
		Signature:  sig,
		Receiver:   recv,
		TypeParams: typeParams(fn.Type.TypeParams),
		Exported:   ast.IsExported(name),
		Tags:       funcTags(fn),
	}
}

func typeSymbol(p *packages.Package, fset *token.FileSet, gen *ast.GenDecl, ts *ast.TypeSpec, pkgID, fileID string) Symbol {
	kind := "type"
	switch ts.Type.(type) {
	case *ast.InterfaceType:
		kind = "iface"
	case *ast.StructType:
		kind = "struct"
	}
	if ts.Assign.IsValid() {
		kind = "alias"
	}

	name := ts.Name.Name
	sig := printNode(fset, ts)
	if gen.Doc != nil && ts.Doc == nil {
		ts.Doc = gen.Doc
	}

	return Symbol{
		ID:         SymbolID(p.PkgPath, kind, name, ""),
		Kind:       kind,
		Name:       name,
		PackageID:  pkgID,
		FileID:     fileID,
		Range:      rangeOf(fset, gen),
		Doc:        docText(ts.Doc),
		Signature:  sig,
		TypeParams: typeParams(ts.TypeParams),
		Exported:   ast.IsExported(name),
	}
}

func valueSymbol(p *packages.Package, fset *token.FileSet, gen *ast.GenDecl, vs *ast.ValueSpec, name *ast.Ident, kind, pkgID, fileID string) Symbol {
	if vs.Doc == nil && gen.Doc != nil {
		vs.Doc = gen.Doc
	}
	return Symbol{
		ID:        SymbolID(p.PkgPath, kind, name.Name, ""),
		Kind:      kind,
		Name:      name.Name,
		PackageID: pkgID,
		FileID:    fileID,
		Range:     rangeOf(fset, gen),
		Doc:       docText(vs.Doc),
		Signature: strings.TrimSpace(printNode(fset, vs)),
		Exported:  ast.IsExported(name.Name),
	}
}

func recvTypeName(expr ast.Expr) (string, bool) {
	switch t := expr.(type) {
	case *ast.StarExpr:
		name, _ := recvTypeName(t.X)
		return name, true
	case *ast.Ident:
		return t.Name, false
	case *ast.IndexExpr:
		name, ptr := recvTypeName(t.X)
		return name, ptr
	case *ast.IndexListExpr:
		name, ptr := recvTypeName(t.X)
		return name, ptr
	}
	return "", false
}

func typeParams(tp *ast.FieldList) []string {
	if tp == nil {
		return nil
	}
	var out []string
	for _, field := range tp.List {
		for _, n := range field.Names {
			out = append(out, n.Name)
		}
	}
	return out
}

func funcTags(fn *ast.FuncDecl) []string {
	var tags []string
	name := fn.Name.Name
	if strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") || strings.HasPrefix(name, "Example") {
		tags = append(tags, strings.ToLower(strings.SplitN(name, "_", 2)[0][:4]))
	}
	if name == "main" {
		tags = append(tags, "main")
	}
	return tags
}

func rangeOf(fset *token.FileSet, n ast.Node) Range {
	start := fset.Position(n.Pos())
	end := fset.Position(n.End())
	return Range{
		StartLine:   start.Line,
		StartCol:    start.Column,
		EndLine:     end.Line,
		EndCol:      end.Column,
		StartOffset: start.Offset,
		EndOffset:   end.Offset,
	}
}

func docText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	return strings.TrimSpace(cg.Text())
}

func printNode(fset *token.FileSet, n ast.Node) string {
	var b strings.Builder
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}
	_ = cfg.Fprint(&b, fset, n)
	return strings.TrimSpace(b.String())
}

func sortIndex(idx *Index) {
	sort.Slice(idx.Packages, func(i, j int) bool {
		return idx.Packages[i].ImportPath < idx.Packages[j].ImportPath
	})
	for i := range idx.Packages {
		sort.Strings(idx.Packages[i].FileIDs)
		sort.Strings(idx.Packages[i].SymbolIDs)
	}
	sort.Slice(idx.Files, func(i, j int) bool {
		return idx.Files[i].Path < idx.Files[j].Path
	})
	sort.Slice(idx.Symbols, func(i, j int) bool {
		a, b := idx.Symbols[i], idx.Symbols[j]
		if a.PackageID != b.PackageID {
			return a.PackageID < b.PackageID
		}
		if a.FileID != b.FileID {
			return a.FileID < b.FileID
		}
		return a.Range.StartOffset < b.Range.StartOffset
	})
	sort.Slice(idx.Refs, func(i, j int) bool {
		a, b := idx.Refs[i], idx.Refs[j]
		if a.FromSymbolID != b.FromSymbolID {
			return a.FromSymbolID < b.FromSymbolID
		}
		if a.ToSymbolID != b.ToSymbolID {
			return a.ToSymbolID < b.ToSymbolID
		}
		return a.Range.StartOffset < b.Range.StartOffset
	})
}
