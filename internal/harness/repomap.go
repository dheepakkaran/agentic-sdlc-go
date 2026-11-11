package harness

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RepoMap is the context-engineering core: instead of sending whole files,
// every role that needs the code layout gets one line per declaration
// (package, types, function signatures, no bodies). Only the files the
// architect selects are later sent in full, to the developer alone.
func RepoMap(root string, dirs []string) (string, error) {
	var files []string
	for _, d := range dirs {
		err := filepath.WalkDir(filepath.Join(root, d), func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !e.IsDir() && strings.HasSuffix(p, ".go") {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(files)
	var b strings.Builder
	fset := token.NewFileSet()
	for _, p := range files {
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			return "", err
		}
		rel, _ := filepath.Rel(root, p)
		b.WriteString("## " + rel + " (package " + f.Name.Name + ")\n")
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				doc := ""
				if d.Doc != nil && ast.IsExported(d.Name.Name) {
					doc = "  // " + oneLine(d.Doc.Text()) + "\n"
				}
				d.Body, d.Doc = nil, nil
				b.WriteString("- " + render(fset, d) + "\n" + doc)
			case *ast.GenDecl:
				if d.Tok == token.IMPORT {
					continue
				}
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.TypeSpec:
						b.WriteString("- type " + s.Name.Name + " " + oneLine(render(fset, s.Type)) + "\n")
					case *ast.ValueSpec:
						for _, n := range s.Names {
							b.WriteString("- " + d.Tok.String() + " " + n.Name + "\n")
						}
					}
				}
			}
		}
	}
	return b.String(), nil
}

func render(fset *token.FileSet, n any) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, n); err != nil {
		return "<unprintable>"
	}
	return buf.String()
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 220 {
		s = s[:220] + "…"
	}
	return s
}

// ReadFiles returns "=== path ===\n<content>" for each existing path.
// Reads go through os.Root, so a path from a model cannot leave the repo.
func ReadFiles(dir string, paths []string) string {
	var b strings.Builder
	root, err := os.OpenRoot(dir)
	if err != nil {
		return ""
	}
	defer func() { _ = root.Close() }()
	for _, p := range paths {
		data, err := root.ReadFile(p)
		if err != nil {
			b.WriteString("=== " + p + " (new file) ===\n\n")
			continue
		}
		b.WriteString("=== " + p + " ===\n" + string(data) + "\n")
	}
	return b.String()
}
