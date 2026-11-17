package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type idlParam struct {
	Type     string
	Name     string
	Optional bool
}

type idl map[string][][]idlParam

var (
	idlComment = regexp.MustCompile(`(?s)//[^\n]*|/\*.*?\*/`)
	idlMethod  = regexp.MustCompile(`(?s)^(?:\[[^\]]*\]\s*)?(.+?)\s+(\w+)\s*\((.*)\)$`)
)

var idlInterface = regexp.MustCompile(`interface\s+(\w+)`)

func readIDL(files ...string) (idl, map[string]string, error) {
	m := idl{}
	exts := map[string]string{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, nil, err
		}
		src := idlComment.ReplaceAllString(string(data), "")
		iface := ""
		for _, stmt := range strings.Split(src, ";") {
			stmt = strings.TrimSpace(stmt)
			if i := strings.LastIndex(stmt, "{"); i >= 0 {
				if im := idlInterface.FindStringSubmatch(stmt[:i]); im != nil {
					iface = im[1]
				}
				stmt = strings.TrimSpace(stmt[i+1:])
			}
			if strings.HasPrefix(stmt, "const ") || strings.HasPrefix(stmt, "readonly ") || strings.HasPrefix(stmt, "attribute ") {
				continue
			}
			sm := idlMethod.FindStringSubmatch(stmt)
			if sm == nil {
				continue
			}
			var params []idlParam
			if args := strings.TrimSpace(sm[3]); args != "" {
				for _, a := range strings.Split(args, ",") {
					a = strings.Join(strings.Fields(a), " ")
					p := idlParam{}
					if strings.HasPrefix(a, "optional ") {
						p.Optional = true
						a = strings.TrimPrefix(a, "optional ")
					}
					if i := strings.Index(a, " ="); i >= 0 {
						a = a[:i]
					}
					a = regexp.MustCompile(`\[[^\]]*\]\s*`).ReplaceAllString(a, "")
					i := strings.LastIndex(a, " ")
					if i < 0 {
						continue
					}
					p.Type, p.Name = a[:i], a[i+1:]
					params = append(params, p)
				}
			}
			m[sm[2]] = append(m[sm[2]], params)
			if extensionInterface.MatchString(iface) {
				exts[sm[2]] = iface
			}
		}
	}
	return m, exts, nil
}

var extensionInterface = regexp.MustCompile(`^[A-Z0-9]+_\w+$`)

var glAliases = map[string]string{
	"EnableiEXT":                "EnableiOES",
	"DisableiEXT":               "DisableiOES",
	"IsEnablediEXT":             "IsEnablediOES",
	"BlendEquationiEXT":         "BlendEquationiOES",
	"BlendEquationSeparateiEXT": "BlendEquationSeparateiOES",
	"BlendFunciEXT":             "BlendFunciOES",
	"BlendFuncSeparateiEXT":     "BlendFuncSeparateiOES",
	"ColorMaskiEXT":             "ColorMaskiOES",
	"GetQueryObjectuivEXT":      "GetQueryObjectuiv",
	"GetGraphicsResetStatusEXT": "GetGraphicsResetStatusKHR",
}

var vendorSuffix = regexp.MustCompile(`(OES|EXT|ANGLE|NV|KHR|APPLE|ARB)$`)

func (p *Package) alias(c *Command) *Command {
	target, ok := glAliases[c.GoName]
	if !ok {
		base := vendorSuffix.ReplaceAllString(c.GoName, "")
		core := p.core[base]
		if base == c.GoName || core == nil || core.Return.GoType() != c.Return.GoType() || len(core.Params) != len(c.Params) {
			return nil
		}
		for i := range core.Params {
			if core.Params[i].Type.GoType() != c.Params[i].Type.GoType() {
				return nil
			}
		}
		target = base
	}
	a := *c
	a.Name = "gl" + target
	t := p.core[target]
	if t == nil {
		t = p.byGoName(target)
	}
	if t != nil && len(t.Params) == len(c.Params) {
		names := map[string]string{}
		for i, tp := range t.Params {
			names[tp.Name] = c.Params[i].Name
		}
		a.Params = make([]Param, len(c.Params))
		for i, cp := range c.Params {
			cp.Class = t.Params[i].Class
			cp.Len = lenWord.ReplaceAllStringFunc(t.Params[i].Len, func(w string) string {
				if n, ok := names[w]; ok {
					return n
				}
				return w
			})
			a.Params[i] = cp
		}
		a.Class = t.Class
	}
	a.Overloads = nil
	return &a
}

var lenWord = regexp.MustCompile(`[A-Za-z_]\w*`)

func (p *Package) byGoName(name string) *Command {
	for _, c := range p.Commands {
		if c.GoName == name {
			return c
		}
	}
	return nil
}

var webglTables = map[string]string{
	"buffer":             "Buffers",
	"texture":            "Textures",
	"framebuffer":        "Framebuffers",
	"renderbuffer":       "Renderbuffers",
	"program":            "Programs",
	"shader":             "Shaders",
	"vertex array":       "VertexArrays",
	"query":              "Queries",
	"sampler":            "Samplers",
	"transform feedback": "TransformFeedbacks",
	"sync":               "Syncs",
}

var genObjects = map[string]string{
	"Buffers":            "Buffer",
	"Framebuffers":       "Framebuffer",
	"Renderbuffers":      "Renderbuffer",
	"Textures":           "Texture",
	"VertexArrays":       "VertexArray",
	"Queries":            "Query",
	"Samplers":           "Sampler",
	"TransformFeedbacks": "TransformFeedback",
}

func jsName(goName string) string {
	return strings.ToLower(goName[:1]) + goName[1:]
}

func handWritten(dir string) (map[string]bool, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.IsExported() {
					names[fd.Name.Name] = true
				}
			}
		}
	}
	return names, nil
}

func scalarIDL(t string) bool {
	switch strings.TrimSuffix(t, "?") {
	case "GLenum", "GLboolean", "GLbitfield", "GLbyte", "GLshort", "GLint", "GLsizei", "GLintptr", "GLsizeiptr",
		"GLubyte", "GLushort", "GLuint", "GLfloat", "GLclampf", "GLint64", "GLuint64":
		return true
	}
	return false
}

func scalarGo(gt string) bool {
	switch gt {
	case "bool", "int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "int", "float32", "float64":
		return true
	}
	return false
}

type jsWriter struct {
	b       bytes.Buffer
	methods []string
	seen    map[string]bool
	stats   map[string]int
	exts    map[string]string
}

func (w *jsWriter) method(name string) string {
	v := "js" + strings.ToUpper(name[:1]) + name[1:]
	if !w.seen[name] {
		w.seen[name] = true
		w.methods = append(w.methods, name)
	}
	return v
}

func returnExpr(t Type, class, v string) (string, bool) {
	switch gt := t.GoType(); {
	case class != "":
		table, ok := webglTables[class]
		if !ok {
			return "", false
		}
		if gt == "uintptr" {
			return "uintptr(webgl." + table + ".Add(" + v + "))", true
		}
		return "webgl." + table + ".Add(" + v + ")", true
	case gt == "bool":
		return "webgl.Bool(" + v + ")", true
	case scalarGo(gt):
		return gt + "(webgl.Number(" + v + "))", true
	}
	return "", false
}

var (
	lenRe        = regexp.MustCompile(`^(?:(\w+)\*)?(\d+)$|^(\w+)$`)
	webglRenames = map[string]string{
		"clearDepthf":                        "clearDepth",
		"depthRangef":                        "depthRange",
		"polygonModeNV":                      "polygonModeWEBGL",
		"drawArraysInstancedBaseInstanceEXT": "drawArraysInstancedBaseInstanceWEBGL",
	}
	listTypes = map[string]string{"Float32List": "*float32", "Int32List": "*int32", "Uint32List": "*uint32"}
)

func lenExpr(l string) (string, bool) {
	m := lenRe.FindStringSubmatch(l)
	switch {
	case m == nil:
		return "", false
	case m[3] != "":
		return "int(" + goName(m[3]) + ")", true
	case m[1] != "":
		return "int(" + goName(m[1]) + ")*" + m[2], true
	}
	return m[2], true
}

func argJS(p Param, it idlParam) (string, bool) {
	n := goName(p.Name)
	gt := p.Type.GoType()
	idlType := strings.TrimSuffix(it.Type, "?")
	if want, ok := listTypes[idlType]; ok {
		l, good := lenExpr(p.Len)
		if gt != want || !good {
			return "", false
		}
		return "webgl." + idlType + "(unsafe.Pointer(" + n + "), " + l + ")", true
	}
	switch {
	case idlType == "WebGLUniformLocation" && p.Name == "location" && gt == "int32":
		return "webgl.Uniform(" + n + ")", true
	case strings.HasPrefix(idlType, "WebGL"):
		table, ok := webglTables[p.Class]
		if !ok {
			return "", false
		}
		if gt == "uintptr" {
			return "webgl." + table + ".Get(uint32(" + n + "))", true
		}
		if gt != "uint32" {
			return "", false
		}
		return "webgl." + table + ".Get(" + n + ")", true
	case scalarIDL(idlType) && scalarGo(gt) && p.Class == "":
		return n, true
	}
	return "", false
}

func (w *jsWriter) auto(c *Command, spec idl) bool {
	name := strings.TrimPrefix(c.Name, "gl")
	if obj, ok := genObjects[strings.TrimPrefix(name, "Gen")]; ok && strings.HasPrefix(name, "Gen") && len(c.Params) == 2 {
		create := "create" + obj
		if _, ok := spec[create]; !ok {
			return false
		}
		fmt.Fprintf(&w.b, "\tgenerated := unsafe.Slice(%s, %s)\n\tfor i := range generated {\n\t\tgenerated[i] = webgl.%s.Add(%s.Invoke())\n\t}\n",
			goName(c.Params[1].Name), goName(c.Params[0].Name), strings.TrimPrefix(name, "Gen"), w.method(create))
		return true
	}
	if obj, ok := genObjects[strings.TrimPrefix(name, "Delete")]; ok && strings.HasPrefix(name, "Delete") && len(c.Params) == 2 {
		table := strings.TrimPrefix(name, "Delete")
		del := "delete" + obj
		if _, ok := spec[del]; !ok {
			return false
		}
		fmt.Fprintf(&w.b, "\tfor _, id := range unsafe.Slice(%s, %s) {\n\t\t%s.Invoke(webgl.%s.Get(id))\n\t\twebgl.%s.Remove(id)\n\t}\n",
			goName(c.Params[1].Name), goName(c.Params[0].Name), w.method(del), table, table)
		return true
	}

	js := jsName(name)
	if r, ok := webglRenames[js]; ok {
		js = r
	}
	counts := map[string]bool{}
	for _, p := range c.Params {
		if m := lenRe.FindStringSubmatch(p.Len); m != nil && m[1] != "" {
			counts[m[1]] = true
		}
	}
	var reduced []Param
	for _, p := range c.Params {
		if !counts[p.Name] {
			reduced = append(reduced, p)
		}
	}
	fits := func(ov []idlParam, n int) bool {
		req := 0
		for _, p := range ov {
			if !p.Optional {
				req++
			}
		}
		return n >= req && n <= len(ov)
	}
	for _, ov := range spec[js] {
		for _, params := range [][]Param{c.Params, reduced} {
			if !fits(ov, len(params)) {
				continue
			}
			var args []string
			ok := true
			for i, p := range params {
				a, good := argJS(p, ov[i])
				if !good {
					ok = false
					break
				}
				args = append(args, a)
			}
			if !ok {
				continue
			}
			return w.invoke(c, name, js, args)
		}
	}
	return false
}

func (w *jsWriter) invoke(c *Command, name, js string, args []string) bool {
	invoke := w.method(js) + ".Invoke(" + strings.Join(args, ", ") + ")"
	if c.Return.IsVoid() {
		w.b.WriteString("\t" + invoke + "\n")
		if strings.HasPrefix(name, "Delete") && len(c.Params) == 1 {
			if table, ok := webglTables[c.Params[0].Class]; ok {
				id := goName(c.Params[0].Name)
				if c.Params[0].Type.GoType() == "uintptr" {
					id = "uint32(" + id + ")"
				}
				fmt.Fprintf(&w.b, "\twebgl.%s.Remove(%s)\n", table, id)
			}
		}
		return true
	}
	r, ok := returnExpr(c.Return, c.Class, invoke)
	if !ok {
		return false
	}
	w.b.WriteString("\treturn " + r + "\n")
	return true
}

func (p *Package) emitWebGL(dir, webglDir string, spec idl) error {
	hand, err := handWritten(webglDir)
	if err != nil {
		return err
	}
	w := &jsWriter{seen: map[string]bool{}, stats: map[string]int{}, exts: p.exts}

	sig := func(name string, params []Param, ret Type) string {
		var decl []string
		for _, prm := range params {
			decl = append(decl, goName(prm.Name)+" "+prm.Type.GoType())
		}
		r := ""
		if !ret.IsVoid() {
			r = " (r " + ret.GoType() + ")"
		}
		return fmt.Sprintf("func %s(%s)%s {\n", name, strings.Join(decl, ", "), r)
	}
	forwardAs := func(name string, params []Param, ret Type) {
		var args []string
		for _, prm := range params {
			args = append(args, goName(prm.Name))
		}
		call := "webgl." + name + "(" + strings.Join(args, ", ") + ")"
		if ret.IsVoid() {
			w.b.WriteString("\t" + call + "\n")
		} else {
			w.b.WriteString("\treturn " + call + "\n")
		}
	}
	forward := func(name string, params []Param, ret Type) {
		var args []string
		for _, prm := range params {
			args = append(args, goName(prm.Name))
		}
		call := "webgl." + name + "(" + strings.Join(args, ", ") + ")"
		if ret.IsVoid() {
			w.b.WriteString("\t" + call + "\n")
		} else {
			w.b.WriteString("\treturn " + call + "\n")
		}
	}

	for _, c := range p.Commands {
		w.b.WriteString("\n" + comment(c, c.Params))
		w.b.WriteString(sig(c.GoName, c.Params, c.Return))
		a := p.alias(c)
		switch {
		case hand[c.GoName]:
			forward(c.GoName, c.Params, c.Return)
			w.stats["hand"]++
		case a != nil && hand[strings.TrimPrefix(a.Name, "gl")]:
			forwardAs(strings.TrimPrefix(a.Name, "gl"), c.Params, c.Return)
			w.stats["alias"]++
		case w.auto(c, spec):
			w.stats["auto"]++
		case a != nil && w.auto(a, spec):
			w.stats["alias"]++
		default:
			w.b.WriteString("\twebgl.Unsupported()\n")
			if !c.Return.IsVoid() {
				w.b.WriteString("\treturn\n")
			}
			w.stats["stub"]++
		}
		w.b.WriteString("}\n")
		for _, o := range c.Overloads {
			w.b.WriteString("\n" + sig(o.GoName, o.Params, c.Return))
			if hand[o.GoName] {
				forward(o.GoName, o.Params, c.Return)
			} else {
				w.b.WriteString("\twebgl.Unsupported()\n")
				if !c.Return.IsVoid() {
					w.b.WriteString("\treturn\n")
				}
			}
			w.b.WriteString("}\n")
		}
	}

	var b bytes.Buffer
	b.WriteString(header)
	b.WriteString("//go:build js\n\npackage " + p.Name + "\n\n")
	b.WriteString("import (\n\t\"errors\"\n\t\"syscall/js\"\n\t\"unsafe\"\n\n\t\"github.com/gen2brain/gl/internal/webgl\"\n)\n\n")
	b.WriteString("var (\n")
	for _, m := range w.methods {
		fmt.Fprintf(&b, "\tjs%s js.Value\n", strings.ToUpper(m[:1])+m[1:])
	}
	b.WriteString(")\n\nvar jsprocs = [...]struct {\n\tv    *js.Value\n\tname string\n\text  string\n}{\n")
	for _, m := range w.methods {
		fmt.Fprintf(&b, "\t{&js%s, %q, %q},\n", strings.ToUpper(m[:1])+m[1:], m, w.exts[m])
	}
	b.WriteString("}\n")
	fmt.Fprintf(&b, `
// Init initializes the bindings with the WebGL2 context in globalThis.goGL.
func Init() error {
	ctx := js.Global().Get("goGL")
	if !ctx.Truthy() {
		return errors.New("%[1]s: globalThis.goGL is not set")
	}
	return InitWithContext(ctx)
}

// InitWithContext initializes the bindings with a WebGL2 rendering context.
func InitWithContext(ctx js.Value) error {
	if err := webgl.Init(ctx); err != nil {
		return err
	}
	bindMethods()
	webgl.OnRestore(bindMethods)
	return nil
}

func bindMethods() {
	for _, p := range jsprocs {
		*p.v = webgl.Method(p.ext, p.name)
	}
}

// InitWithProcAddrFunc is not supported in the browser.
func InitWithProcAddrFunc(getProcAddr func(name string) unsafe.Pointer) error {
	return errors.New("%[1]s: InitWithProcAddrFunc is not supported on js")
}
`, p.Name)
	b.Write(w.b.Bytes())
	p.webglStats = fmt.Sprintf("webgl: %d auto, %d hand-written, %d aliased, %d unsupported", w.stats["auto"], w.stats["hand"], w.stats["alias"], w.stats["stub"])
	return writeGo(filepath.Join(dir, "funcs_js.go"), &b)
}
