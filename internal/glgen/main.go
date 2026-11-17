// Command glgen generates the OpenGL packages from the Khronos registry.
package main

import (
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var packages = []PackageSpec{
	{Dir: "v2.1/gl", API: "gl", Version: "2.1"},
	{Dir: "all-core/gl", API: "gl", Version: "all", Profile: "core", Lenient: true},
	{Dir: "v3.2-core/gl", API: "gl", Version: "3.2", Profile: "core"},
	{Dir: "v3.3-core/gl", API: "gl", Version: "3.3", Profile: "core"},
	{Dir: "v4.1-core/gl", API: "gl", Version: "4.1", Profile: "core"},
	{Dir: "v4.2-core/gl", API: "gl", Version: "4.2", Profile: "core"},
	{Dir: "v4.3-core/gl", API: "gl", Version: "4.3", Profile: "core"},
	{Dir: "v4.4-core/gl", API: "gl", Version: "4.4", Profile: "core"},
	{Dir: "v4.5-core/gl", API: "gl", Version: "4.5", Profile: "core"},
	{Dir: "v4.6-core/gl", API: "gl", Version: "4.6", Profile: "core"},
	{Dir: "v3.2-compatibility/gl", API: "gl", Version: "3.2", Profile: "compatibility"},
	{Dir: "v3.3-compatibility/gl", API: "gl", Version: "3.3", Profile: "compatibility"},
	{Dir: "v4.1-compatibility/gl", API: "gl", Version: "4.1", Profile: "compatibility"},
	{Dir: "v4.2-compatibility/gl", API: "gl", Version: "4.2", Profile: "compatibility"},
	{Dir: "v4.3-compatibility/gl", API: "gl", Version: "4.3", Profile: "compatibility"},
	{Dir: "v4.4-compatibility/gl", API: "gl", Version: "4.4", Profile: "compatibility"},
	{Dir: "v4.5-compatibility/gl", API: "gl", Version: "4.5", Profile: "compatibility"},
	{Dir: "v4.6-compatibility/gl", API: "gl", Version: "4.6", Profile: "compatibility"},
	{Dir: "v3.0/gles2", API: "gles2", Version: "3.0", WebGL: true},
	{Dir: "v3.1/gles2", API: "gles2", Version: "3.1"},
	{Dir: "v3.2/gles2", API: "gles2", Version: "3.2"},
}

func main() {
	var (
		root    = flag.String("out", ".", "output root directory")
		regfile = flag.String("spec", "internal/glgen/xml/gl.xml", "gl.xml registry")
		ovl     = flag.String("overload", "internal/glgen/xml/overload.xml", "overload definitions")
		docs    = flag.String("docs", "internal/glgen/xml/docs.tsv", "function documentation table")
		refdocs = flag.String("refpages", "", "rebuild the documentation table from a directory of OpenGL refpages")
		only    = flag.String("only", "", "generate only the package in this directory")
	)
	flag.Parse()

	if *refdocs != "" {
		if err := buildDocs(*refdocs, *docs); err != nil {
			log.Fatal(err)
		}
	}

	s, err := readSpec(*regfile, *ovl)
	if err != nil {
		log.Fatal(err)
	}
	doc, err := readDocs(*docs)
	if err != nil {
		log.Fatal(err)
	}
	spec, exts, err := readIDL(filepath.Join(filepath.Dir(*ovl), "webgl.idl"), filepath.Join(filepath.Dir(*ovl), "webgl2.idl"), filepath.Join(filepath.Dir(*ovl), "webgl_ext.idl"))
	if err != nil {
		log.Fatal(err)
	}

	for _, ps := range packages {
		if *only != "" && ps.Dir != *only {
			continue
		}
		p, err := s.Package(ps)
		if err != nil {
			log.Fatal(err)
		}
		p.idl, p.exts, p.webglDir = spec, exts, "internal/webgl"
		for _, c := range p.Commands {
			c.Doc = doc[c.Name]
		}
		if err := p.generate(*root); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%-24s %5d commands %5d enums\n", ps.Dir, len(p.Commands), len(p.Enums))
	}
}

func readDocs(file string) (map[string]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	docs := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name, doc, ok := strings.Cut(sc.Text(), "\t")
		if ok {
			docs[name] = doc
		}
	}
	return docs, sc.Err()
}

func buildDocs(dir, out string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.xml"))
	if err != nil {
		return err
	}
	docs := map[string]string{}
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		var d struct {
			Purpose string   `xml:"refnamediv>refpurpose"`
			Names   []string `xml:"refnamediv>refname"`
		}
		dec := xml.NewDecoder(f)
		dec.Strict = false
		err = dec.Decode(&d)
		f.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		purpose := strings.Join(strings.Fields(d.Purpose), " ")
		for _, n := range d.Names {
			docs[n] = purpose
		}
	}
	var b strings.Builder
	for _, k := range slices.Sorted(maps.Keys(docs)) {
		fmt.Fprintf(&b, "%s\t%s\n", k, docs[k])
	}
	return os.WriteFile(out, []byte(b.String()), 0o644)
}
