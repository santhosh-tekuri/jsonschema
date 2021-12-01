package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/santhosh-tekuri/jsonschema/v5"
	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader"
)

func usage() {
	fmt.Fprintln(os.Stderr, "jv [-draft INT] [-output FORMAT] <json-schema> [<json-doc>]...")
	flag.PrintDefaults()
}

func main() {
	draft := flag.Int("draft", 2020, "draft used when '$schema' attribute is missing. valid values 4, 5, 7, 2019, 2020")
	output := flag.String("output", "", "output format. valid values flag, basic, detailed")
	flag.Usage = usage
	flag.Parse()
	if len(flag.Args()) == 0 {
		usage()
		os.Exit(1)
	}

	compiler := jsonschema.NewCompiler()
	switch *draft {
	case 4:
		compiler.Draft = jsonschema.Draft4
	case 6:
		compiler.Draft = jsonschema.Draft6
	case 7:
		compiler.Draft = jsonschema.Draft7
	case 2019:
		compiler.Draft = jsonschema.Draft2019
	case 2020:
		compiler.Draft = jsonschema.Draft2020
	default:
		fmt.Fprintln(os.Stderr, "draft must be 4, 5, 7, 2019 or 2020")
		os.Exit(1)
	}

	var validOutput bool
	for _, out := range []string{"", "flag", "basic", "detailed"} {
		if *output == out {
			validOutput = true
			break
		}
	}
	if !validOutput {
		fmt.Fprintln(os.Stderr, "output must be flag, basic or detailed")
		os.Exit(1)
	}

	schema, err := compiler.Compile(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%#v\n", err)
		os.Exit(1)
	}

	for _, f := range flag.Args()[1:] {
		validate(schema, *output, f)
	}
}

func validate(schema *jsonschema.Schema, output, f string) {
	file, err := os.Open(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	var v interface{}

	switch {
	case strings.HasSuffix(f, ".yaml"),
		strings.HasSuffix(f, ".yml"):
		dec := yaml.NewDecoder(file)
		if err = dec.Decode(&v); err != nil {
			fmt.Fprintf(os.Stderr, "invalid yaml file %s: %v", f, err)
		}
		v, err = toStringKeys(v)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error converting yaml keys to strings %s: %v", f, err)
		}
	default:
		dec := json.NewDecoder(file)
		dec.UseNumber()
		if err = dec.Decode(&v); err != nil {
			fmt.Fprintf(os.Stderr, "invalid json file %s: %v", f, err)
		}
	}

	err = schema.Validate(v)
	if err != nil {
		if ve, ok := err.(*jsonschema.ValidationError); ok {
			var out interface{}
			switch output {
			case "flag":
				out = ve.FlagOutput()
			case "basic":
				out = ve.BasicOutput()
			case "detailed":
				out = ve.DetailedOutput()
			}
			if out == nil {
				fmt.Fprintf(os.Stderr, "%v: %v\n", f, err)
			} else {
				b, _ := json.MarshalIndent(out, "", "  ")
				fmt.Fprintln(os.Stderr, string(b))
			}
		} else {
			fmt.Fprintf(os.Stderr, "validation error %v: %v\n", f, err)
		}
		os.Exit(1)
	}
}

func toStringKeys(val interface{}) (interface{}, error) {
	var err error
	switch val := val.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for k, v := range val {
			k, ok := k.(string)
			if !ok {
				return nil, errors.New("found non-string key")
			}
			m[k], err = toStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return m, nil
	case []interface{}:
		var l = make([]interface{}, len(val))
		for i, v := range val {
			l[i], err = toStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return l, nil
	default:
		return val, nil
	}
}
