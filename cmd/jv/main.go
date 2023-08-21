package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader"
	"gopkg.in/yaml.v3"
)

func usage() {
	fmt.Fprintln(os.Stderr, "jv [-draft INT] [-output FORMAT] [-assertformat] [-assertcontent] <json-schema> [<json-or-yaml-doc>]...")
	flag.PrintDefaults()
}

var (
	validDrafts = map[int]*jsonschema.Draft{
		4:    jsonschema.Draft4,
		6:    jsonschema.Draft6,
		7:    jsonschema.Draft7,
		2019: jsonschema.Draft2019,
		2020: jsonschema.Draft2020,
	}
	validOutputs = []string{"flag", "basic", "detailed"}
)

func main() {
	drafts := func() string {
		ds := make([]int, 0, len(validDrafts))
		for d := range validDrafts {
			ds = append(ds, d)
		}
		sort.Ints(ds)
		var b strings.Builder
		for i, d := range ds {
			if i != 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%d", d)
		}
		return b.String()
	}()
	draft := flag.Int("draft", 2020, "draft used when '$schema' attribute is missing. valid values "+drafts)
	output := flag.String("output", "", "output format. valid values "+strings.Join(validOutputs, ", "))
	assertFormat := flag.Bool("assertformat", false, "enable format assertions with draft >= 2019")
	assertContent := flag.Bool("assertcontent", false, "enable content assertions with draft >= 2019")
	flag.Usage = usage
	flag.Parse()
	if len(flag.Args()) == 0 {
		usage()
		os.Exit(2)
	}

	compiler := jsonschema.NewCompiler()
	var ok bool
	if compiler.Draft, ok = validDrafts[*draft]; !ok {
		fmt.Fprintln(os.Stderr, "draft must be one of", drafts)
		os.Exit(2)
	}

	compiler.LoadURL = loadURL
	compiler.AssertFormat = *assertFormat
	compiler.AssertContent = *assertContent

	if *output != "" {
		valid := false
		for _, out := range validOutputs {
			if *output == out {
				valid = true
				break
			}
		}
		if !valid {
			fmt.Fprintln(os.Stderr, "output must be one of", strings.Join(validOutputs, ", "))
			os.Exit(2)
		}
	}

	schema, err := compiler.Compile(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%#v\n", err)
		os.Exit(1)
	}

	exitCode := 0
	for _, f := range flag.Args()[1:] {
		file, err := os.Open(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			exitCode = 1
			continue
		}
		defer file.Close()

		v, err := decodeFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			exitCode = 1
			continue
		}

		err = schema.Validate(v)
		if err != nil {
			exitCode = 1
			if ve, ok := err.(*jsonschema.ValidationError); ok {
				var out interface{}
				switch *output {
				case "flag":
					out = ve.FlagOutput()
				case "basic":
					out = ve.BasicOutput()
				case "detailed":
					out = ve.DetailedOutput()
				}
				if out == nil {
					fmt.Fprintf(os.Stderr, "%#v\n", err)
				} else {
					b, _ := json.MarshalIndent(out, "", "  ")
					fmt.Fprintln(os.Stderr, string(b))
				}
			} else {
				fmt.Fprintf(os.Stderr, "validation failed: %v\n", err)
			}
		} else {
			if *output != "" {
				fmt.Println("{\n  \"valid\": true\n}")
			}
		}
	}
	os.Exit(exitCode)
}

func loadURL(s string) (io.ReadCloser, error) {
	r, err := jsonschema.LoadURL(s)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(s, ".yaml") || strings.HasSuffix(s, ".yml") {
		defer r.Close()
		v, err := decodeYAML(r, s)
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return ioutil.NopCloser(bytes.NewReader(b)), nil
	}
	return r, err
}

func decodeFile(file *os.File) (interface{}, error) {
	ext := filepath.Ext(file.Name())
	if ext == ".yaml" || ext == ".yml" {
		return decodeYAML(file, file.Name())
	}

	// json file
	var v interface{}
	dec := json.NewDecoder(file)
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("invalid json file %s: %v", file.Name(), err)
	}
	return v, nil
}

func decodeYAML(r io.Reader, name string) (interface{}, error) {
	var v interface{}
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("invalid yaml file %s: %v", name, err)
	}
	return v, nil
}
