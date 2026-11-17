package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type api struct {
	funcs  map[string]string
	consts map[string]string
}

func exprString(fset *token.FileSet, e ast.Expr) string {
	var b bytes.Buffer
	printer.Fprint(&b, fset, e)
	return strings.ReplaceAll(b.String(), "interface{}", "any")
}

func readAPI(t *testing.T, dir string, js bool) api {
	fset := token.NewFileSet()
	a := api{funcs: map[string]string{}, consts: map[string]string{}}
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	for _, file := range files {
		isJS := strings.HasSuffix(file, "_js.go")
		native := strings.HasSuffix(file, "funcs.go") || strings.HasSuffix(file, "debug_native.go") || strings.Contains(file, "funcs_wide")
		if (js && native) || (!js && isJS) {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil || !d.Name.IsExported() {
					continue
				}
				var sig []string
				for _, p := range d.Type.Params.List {
					n := max(len(p.Names), 1)
					for range n {
						sig = append(sig, exprString(fset, p.Type))
					}
				}
				s := "(" + strings.Join(sig, ", ") + ")"
				if d.Type.Results != nil {
					var res []string
					for _, r := range d.Type.Results.List {
						n := max(len(r.Names), 1)
						for range n {
							res = append(res, exprString(fset, r.Type))
						}
					}
					s += " (" + strings.Join(res, ", ") + ")"
				}
				a.funcs[d.Name.Name] = s
			case *ast.GenDecl:
				if d.Tok != token.CONST {
					continue
				}
				for _, sp := range d.Specs {
					vs := sp.(*ast.ValueSpec)
					for i, n := range vs.Names {
						a.consts[n.Name] = exprString(fset, vs.Values[i])
					}
				}
			}
		}
	}
	return a
}

var registryChanged = map[string]bool{
	"ReleaseClientPointerRangeMESA": true,
}

func TestGoGLCompat(t *testing.T) {
	gogl := os.Getenv("GLGEN_GOGL")
	if gogl == "" {
		t.Skip("GLGEN_GOGL not set to a github.com/go-gl/gl checkout")
	}
	s, err := readSpec("xml/gl.xml", "xml/overload.xml")
	if err != nil {
		t.Fatal(err)
	}
	spec, exts, err := readIDL("xml/webgl.idl", "xml/webgl2.idl", "xml/webgl_ext.idl")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	for _, ps := range packages {
		t.Run(ps.Dir, func(t *testing.T) {
			p, err := s.Package(ps)
			if err != nil {
				t.Fatal(err)
			}
			p.idl, p.exts, p.webglDir = spec, exts, "../webgl"
			if err := p.generate(out); err != nil {
				t.Fatal(err)
			}
			want := readAPI(t, filepath.Join(gogl, ps.Dir), false)
			got := readAPI(t, filepath.Join(out, ps.Dir), false)
			for name, sig := range want.funcs {
				if g, ok := got.funcs[name]; !ok {
					t.Errorf("missing func %s%s", name, sig)
				} else if g != sig && !registryChanged[name] {
					t.Errorf("func %s: got %s, want %s", name, g, sig)
				}
			}
			for name, v := range want.consts {
				if g, ok := got.consts[name]; !ok {
					t.Errorf("missing const %s = %s", name, v)
				} else if g != v {
					t.Errorf("const %s: got %s, want %s", name, g, v)
				}
			}
			extra := 0
			for name := range got.funcs {
				if _, ok := want.funcs[name]; !ok {
					extra++
				}
			}
			t.Logf("%d funcs, %d consts, %d new funcs", len(got.funcs), len(got.consts), extra)
		})
	}
}

func TestWebGLParity(t *testing.T) {
	s, err := readSpec("xml/gl.xml", "xml/overload.xml")
	if err != nil {
		t.Fatal(err)
	}
	spec, exts, err := readIDL("xml/webgl.idl", "xml/webgl2.idl", "xml/webgl_ext.idl")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	for _, ps := range packages {
		if !ps.WebGL {
			continue
		}
		p, err := s.Package(ps)
		if err != nil {
			t.Fatal(err)
		}
		p.idl, p.exts, p.webglDir = spec, exts, "../webgl"
		if err := p.generate(out); err != nil {
			t.Fatal(err)
		}
		native := readAPI(t, filepath.Join(out, ps.Dir), false)
		web := readAPI(t, filepath.Join(out, ps.Dir), true)
		for name, sig := range native.funcs {
			if name == "InitWithContext" {
				continue
			}
			if w, ok := web.funcs[name]; !ok {
				t.Errorf("%s: %s missing on js", ps.Dir, name)
			} else if w != sig {
				t.Errorf("%s: %s is %s on js, %s natively", ps.Dir, name, w, sig)
			}
		}
		for name := range web.funcs {
			if _, ok := native.funcs[name]; !ok && name != "InitWithContext" {
				t.Errorf("%s: %s only on js", ps.Dir, name)
			}
		}
	}
}
