package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type xmlRegistry struct {
	Enums      []xmlEnumSet   `xml:"enums"`
	Commands   []xmlCommand   `xml:"commands>command"`
	Features   []xmlFeature   `xml:"feature"`
	Extensions []xmlExtension `xml:"extensions>extension"`
}

type xmlEnumSet struct {
	Enums []xmlEnum `xml:"enum"`
}

type xmlEnum struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
	API   string `xml:"api,attr"`
}

type xmlCommand struct {
	Proto  xmlProto   `xml:"proto"`
	Params []xmlParam `xml:"param"`
}

type xmlProto struct {
	Raw   []byte `xml:",innerxml"`
	Class string `xml:"class,attr"`
}

type xmlParam struct {
	Raw   []byte `xml:",innerxml"`
	Class string `xml:"class,attr"`
	Len   string `xml:"len,attr"`
}

type xmlFeature struct {
	API      string       `xml:"api,attr"`
	Number   string       `xml:"number,attr"`
	Requires []xmlRequire `xml:"require"`
	Removes  []xmlRequire `xml:"remove"`
}

type xmlRequire struct {
	Profile  string   `xml:"profile,attr"`
	Enums    []xmlRef `xml:"enum"`
	Commands []xmlRef `xml:"command"`
}

type xmlRef struct {
	Name string `xml:"name,attr"`
}

type xmlExtension struct {
	Name      string       `xml:"name,attr"`
	Supported string       `xml:"supported,attr"`
	Requires  []xmlRequire `xml:"require"`
	Removes   []xmlRequire `xml:"remove"`
}

type xmlOverloads struct {
	Overloads []struct {
		Name         string `xml:"name,attr"`
		OverloadName string `xml:"overloadName,attr"`
		Changes      []struct {
			Index int `xml:"index,attr"`
			Name  *struct {
				Value string `xml:"value,attr"`
			} `xml:"name"`
			Type *struct {
				Signature string `xml:"signature,attr"`
			} `xml:"type"`
		} `xml:"parameterChanges>change"`
	} `xml:"overload"`
}

// Type is a C type of a parameter or return value.
type Type struct {
	Name     string
	Pointers int
	CDef     string
}

// Param is a command parameter.
type Param struct {
	Name  string
	Type  Type
	Class string
	Len   string
}

// Command is a GL command.
type Command struct {
	Name      string
	GoName    string
	Params    []Param
	Return    Type
	Class     string
	Overloads []Overload
	Doc       string
	Required  bool
}

// Overload is an alternative Go signature of a command.
type Overload struct {
	GoName string
	Params []Param
}

// Enum is a GL enum.
type Enum struct {
	Name   string
	GoName string
	Value  string
}

type addRem struct {
	profile                string
	addCmds, addEnums      []string
	removeCmds, removeEnum []string
}

type feature struct {
	api     string
	version version
	sets    []addRem
}

type extension struct {
	name      string
	supported *regexp.Regexp
	sets      []addRem
}

type version struct{ major, minor int }

func (v version) all() bool { return v.major < 0 }

func (v version) compare(o version) int {
	if v.all() || o.all() {
		return 0
	}
	if v.major != o.major {
		return v.major - o.major
	}
	return v.minor - o.minor
}

func parseVersion(s string) (version, error) {
	if s == "all" {
		return version{-1, -1}, nil
	}
	a, b, ok := strings.Cut(s, ".")
	if !ok {
		return version{}, fmt.Errorf("invalid version %q", s)
	}
	major, err := strconv.Atoi(a)
	if err != nil {
		return version{}, err
	}
	minor, err := strconv.Atoi(b)
	if err != nil {
		return version{}, err
	}
	return version{major, minor}, nil
}

type ref struct{ name, api string }

// Spec is a parsed registry.
type Spec struct {
	commands   map[string]*Command
	enums      map[ref]*Enum
	features   []feature
	extensions []extension
}

func readSpec(file, overloadFile string) (*Spec, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var reg xmlRegistry
	if err := xml.Unmarshal(data, &reg); err != nil {
		return nil, err
	}

	s := &Spec{commands: map[string]*Command{}, enums: map[ref]*Enum{}}
	for _, c := range reg.Commands {
		name, ret, err := parseSignature(c.Proto.Raw)
		if err != nil {
			return nil, err
		}
		cmd := &Command{Name: name, GoName: trimPrefix(name), Return: ret, Class: c.Proto.Class}
		for _, p := range c.Params {
			pname, ptype, err := parseSignature(p.Raw)
			if err != nil {
				return nil, err
			}
			cmd.Params = append(cmd.Params, Param{Name: pname, Type: ptype, Class: p.Class, Len: p.Len})
		}
		s.commands[name] = cmd
	}

	for _, set := range reg.Enums {
		for _, e := range set.Enums {
			s.enums[ref{e.Name, e.API}] = &Enum{Name: e.Name, GoName: trimPrefix(e.Name), Value: e.Value}
		}
	}

	for _, f := range reg.Features {
		v, err := parseVersion(f.Number)
		if err != nil {
			return nil, err
		}
		s.features = append(s.features, feature{api: f.API, version: v, sets: parseAddRem(f.Requires, f.Removes)})
	}

	for _, e := range reg.Extensions {
		if len(e.Removes) > 0 {
			return nil, fmt.Errorf("extension %s removes symbols", e.Name)
		}
		s.extensions = append(s.extensions, extension{
			name:      e.Name,
			supported: regexp.MustCompile("^(" + e.Supported + ")$"),
			sets:      parseAddRem(e.Requires, nil),
		})
	}

	if overloadFile != "" {
		data, err := os.ReadFile(overloadFile)
		if err != nil {
			return nil, err
		}
		var ov xmlOverloads
		if err := xml.Unmarshal(data, &ov); err != nil {
			return nil, err
		}
		for _, o := range ov.Overloads {
			cmd := s.commands[o.Name]
			if cmd == nil {
				return nil, fmt.Errorf("overload of unknown command %s", o.Name)
			}
			params := slices.Clone(cmd.Params)
			for _, c := range o.Changes {
				if c.Index < 0 || c.Index >= len(params) {
					return nil, fmt.Errorf("overload %s: invalid index %d", o.OverloadName, c.Index)
				}
				p := &params[c.Index]
				if c.Type != nil {
					_, t, err := parseSignature([]byte(c.Type.Signature))
					if err != nil {
						return nil, err
					}
					p.Type = t
				}
				if c.Name != nil {
					p.Name = c.Name.Value
				}
			}
			cmd.Overloads = append(cmd.Overloads, Overload{GoName: o.OverloadName, Params: params})
		}
	}
	return s, nil
}

func parseAddRem(requires, removes []xmlRequire) []addRem {
	byProfile := map[string]*addRem{}
	var order []string
	get := func(profile string) *addRem {
		a, ok := byProfile[profile]
		if !ok {
			a = &addRem{profile: profile}
			byProfile[profile] = a
			order = append(order, profile)
		}
		return a
	}
	for _, r := range requires {
		a := get(r.Profile)
		for _, c := range r.Commands {
			a.addCmds = append(a.addCmds, c.Name)
		}
		for _, e := range r.Enums {
			a.addEnums = append(a.addEnums, e.Name)
		}
	}
	for _, r := range removes {
		a := get(r.Profile)
		for _, c := range r.Commands {
			a.removeCmds = append(a.removeCmds, c.Name)
		}
		for _, e := range r.Enums {
			a.removeEnum = append(a.removeEnum, e.Name)
		}
	}
	sets := make([]addRem, 0, len(order))
	for _, p := range order {
		sets = append(sets, *byProfile[p])
	}
	return sets
}

var arrayRe = regexp.MustCompile(`\[.*]`)

func parseSignature(raw []byte) (name string, t Type, err error) {
	inName, inType := false, false
	d := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", t, err
		}
		switch tok := tok.(type) {
		case xml.CharData:
			s := strings.Trim(string(tok), " ")
			if inName {
				name = s
			} else if inType {
				t.Name = s
			}
			if !inName {
				t.CDef += string(tok)
				t.Pointers += strings.Count(s, "*")
			}
		case xml.StartElement:
			switch tok.Name.Local {
			case "ptype":
				inType = true
			case "name":
				inName = true
			default:
				return "", t, fmt.Errorf("unexpected signature XML: %s", raw)
			}
		case xml.EndElement:
			switch tok.Name.Local {
			case "ptype":
				inType = false
			case "name":
				inName = false
			}
		}
	}
	if t.Name == "" {
		n := strings.ReplaceAll(t.CDef, "const", "")
		n = strings.ReplaceAll(n, "*", "")
		t.Name = strings.ReplaceAll(n, " ", "")
	}
	t.CDef = arrayRe.ReplaceAllStringFunc(t.CDef, func(string) string {
		t.Pointers++
		return "*"
	})
	t.CDef = strings.TrimSpace(t.CDef)
	return name, t, nil
}

func trimPrefix(name string) string {
	for _, p := range []string{"glX", "wgl", "egl", "gl", "GLX_", "WGL_", "EGL_", "GL_"} {
		if strings.HasPrefix(name, p) {
			t := strings.TrimPrefix(name, p)
			if strings.IndexAny(t, "0123456789") == 0 {
				return name
			}
			return t
		}
	}
	return name
}

func (s *Spec) enum(name, api string) *Enum {
	if e, ok := s.enums[ref{name, api}]; ok {
		return e
	}
	return s.enums[ref{name, ""}]
}

// PackageSpec selects the symbols of a generated package.
type PackageSpec struct {
	Dir     string
	API     string
	Version string
	Profile string
	Lenient bool
	WebGL   bool
}

// Package is a selected set of commands and enums.
type Package struct {
	PackageSpec
	idl      idl
	exts     map[string]string
	webglDir string
	core     map[string]*Command
	Name     string
	Commands []*Command
	Enums    []*Enum
}

func (s *Spec) Package(ps PackageSpec) (*Package, error) {
	v, err := parseVersion(ps.Version)
	if err != nil {
		return nil, err
	}
	found := false
	for _, f := range s.features {
		if f.api == ps.API && f.version.compare(v) == 0 {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("no feature %s %s", ps.API, ps.Version)
	}

	cmds := map[string]*Command{}
	enums := map[string]*Enum{}
	for _, f := range s.features {
		if f.api != ps.API || v.compare(f.version) < 0 {
			continue
		}
		for _, set := range f.sets {
			if set.profile != "" && set.profile != ps.Profile {
				continue
			}
			for _, c := range set.addCmds {
				cmd := *s.commands[c]
				cmd.Required = !ps.Lenient
				cmds[c] = &cmd
			}
			for _, e := range set.addEnums {
				enums[e] = s.enum(e, ps.API)
			}
			if !v.all() {
				for _, c := range set.removeCmds {
					delete(cmds, c)
				}
				for _, e := range set.removeEnum {
					delete(enums, e)
				}
			}
		}
	}

	extCmds := map[string]bool{}
	extAPI := ps.API
	if ps.API == "gl" && ps.Profile == "core" {
		extAPI = "glcore"
	}
	for _, x := range s.extensions {
		if !x.supported.MatchString(extAPI) {
			continue
		}
		for _, set := range x.sets {
			if set.profile != "" && set.profile != ps.Profile {
				continue
			}
			for _, c := range set.addCmds {
				if _, ok := cmds[c]; !ok {
					cmd := *s.commands[c]
					cmd.Required = false
					cmds[c] = &cmd
					extCmds[c] = true
				}
			}
			for _, e := range set.addEnums {
				enums[e] = s.enum(e, ps.API)
			}
		}
	}

	p := &Package{PackageSpec: ps, Name: ps.API, core: map[string]*Command{}}
	for name, c := range cmds {
		if c.Required || ps.Lenient {
			if _, isExt := extCmds[name]; !isExt {
				p.core[c.GoName] = c
			}
		}
	}
	for _, k := range slices.Sorted(maps.Keys(cmds)) {
		p.Commands = append(p.Commands, cmds[k])
	}
	for _, k := range slices.Sorted(maps.Keys(enums)) {
		p.Enums = append(p.Enums, enums[k])
	}
	return p, nil
}
