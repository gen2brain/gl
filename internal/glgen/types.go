package main

import (
	"fmt"
	"strings"
)

// GoType returns the Go type of t, compatible with go-gl.
func (t Type) GoType() string {
	p := strings.Repeat("*", t.Pointers)
	switch t.Name {
	case "GLbyte":
		return p + "int8"
	case "GLubyte":
		return p + "uint8"
	case "GLshort":
		return p + "int16"
	case "GLushort":
		return p + "uint16"
	case "GLint":
		return p + "int32"
	case "GLuint":
		return p + "uint32"
	case "GLint64", "GLint64EXT":
		return p + "int64"
	case "GLuint64", "GLuint64EXT":
		return p + "uint64"
	case "GLfloat", "GLclampf":
		return p + "float32"
	case "GLdouble", "GLclampd":
		return p + "float64"
	case "GLclampx", "GLsizei", "GLfixed":
		return p + "int32"
	case "GLchar", "GLcharARB":
		return p + "uint8"
	case "GLboolean":
		return p + "bool"
	case "GLenum", "GLbitfield":
		return p + "uint32"
	case "GLhalf", "GLhalfNV":
		return p + "uint16"
	case "void", "GLvoid":
		switch t.Pointers {
		case 1:
			return "unsafe.Pointer"
		case 2:
			return "*unsafe.Pointer"
		}
	case "GLintptr", "GLintptrARB", "GLsizeiptr", "GLsizeiptrARB":
		return p + "int"
	case "GLhandleARB", "GLeglImagesOES", "GLvdpauSurfaceNV", "GLsync", "uintptr_t":
		return p + "uintptr"
	case "GLDEBUGPROC", "GLDEBUGPROCARB", "GLDEBUGPROCKHR":
		return "DebugProc"
	}
	return "unsafe.Pointer"
}

// IsVoid reports whether t is void.
func (t Type) IsVoid() bool {
	return (t.Name == "void" || t.Name == "GLvoid") && t.Pointers == 0
}

// IsDebugProc reports whether t is a debug callback.
func (t Type) IsDebugProc() bool {
	return t.GoType() == "DebugProc"
}

// Kind returns the internal/call argument kind of t.
func (t Type) Kind() (string, error) {
	if t.Pointers > 0 || t.GoType() == "unsafe.Pointer" || t.IsDebugProc() {
		return "PTR", nil
	}
	switch t.Name {
	case "GLbyte", "GLubyte", "GLchar", "GLcharARB", "GLboolean":
		return "I8", nil
	case "GLshort", "GLushort", "GLhalf", "GLhalfNV":
		return "I16", nil
	case "GLint", "GLuint", "GLsizei", "GLenum", "GLbitfield", "GLfixed", "GLclampx":
		return "I32", nil
	case "GLint64", "GLuint64", "GLint64EXT", "GLuint64EXT":
		return "I64", nil
	case "GLintptr", "GLsizeiptr", "GLintptrARB", "GLsizeiptrARB", "GLsync", "GLhandleARB", "GLvdpauSurfaceNV", "uintptr_t":
		return "PTR", nil
	case "GLfloat", "GLclampf":
		return "F32", nil
	case "GLdouble", "GLclampd":
		return "F64", nil
	}
	return "", fmt.Errorf("unknown C type %q", t.Name)
}

var kindBits = map[string]uint64{"I8": 0, "I16": 1, "I32": 2, "I64": 3, "PTR": 4, "F32": 6, "F64": 7}

func (t Type) wide() bool {
	switch t.GoType() {
	case "int64", "uint64", "float64":
		return true
	}
	return false
}

func wideParams(params []Param) bool {
	for _, p := range params {
		if p.Type.wide() {
			return true
		}
	}
	return false
}

func goName(name string) string {
	switch name {
	case "func", "type", "struct", "range", "map", "string":
		return "x" + name
	}
	return name
}

// argExprs returns the uintptr expressions passing p to C, two words for 64-bit scalars when split.
func argExprs(p Param, split bool) []string {
	n := goName(p.Name)
	switch gt := p.Type.GoType(); {
	case gt == "DebugProc":
		return []string{"debugCallback(" + n + ")"}
	case gt == "bool":
		return []string{"boolToUintptr(" + n + ")"}
	case gt == "float32":
		return []string{"uintptr(math.Float32bits(" + n + "))"}
	case gt == "float64" && split:
		return []string{"uintptr(math.Float64bits(" + n + "))", "uintptr(math.Float64bits(" + n + ") >> 32)"}
	case gt == "float64":
		return []string{"uintptr(math.Float64bits(" + n + "))"}
	case (gt == "int64" || gt == "uint64") && split:
		return []string{"uintptr(" + n + ")", "uintptr(" + n + " >> 32)"}
	case gt == "unsafe.Pointer":
		return []string{"uintptr(" + n + ")"}
	case strings.HasPrefix(gt, "*"):
		return []string{"uintptr(unsafe.Pointer(" + n + "))"}
	}
	return []string{"uintptr(" + n + ")"}
}

// descriptor returns the internal/call descriptor words of params.
func descriptor(params []Param, words int) (string, error) {
	if len(params) > 32 {
		return "", fmt.Errorf("%d parameters", len(params))
	}
	var d [2]uint64
	for i, p := range params {
		k, err := p.Type.Kind()
		if err != nil {
			return "", err
		}
		d[i/16] |= kindBits[k] << (4 * (i % 16))
	}
	if words > 16 {
		return fmt.Sprintf("%#x, %#x", d[0], d[1]), nil
	}
	return fmt.Sprintf("%#x", d[0]), nil
}
