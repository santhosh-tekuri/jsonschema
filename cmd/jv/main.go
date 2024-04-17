package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

func main() {
	flag.Usage = func() {
		eprintln("Usage: jv [OPTIONS] SCHEMA [INSTANCE...]")
		eprintln("")
		eprintln("Options:")
		flag.PrintDefaults()
	}
	help := flag.Bool("h", false, "Print help information")
	quiet := flag.Bool("q", false, "Do not print errors")
	draftVersion := flag.Int("d", 2020, "Draft `version` used when '$schema' is missing. Valid values 4, 6, 7, 2019, 2020")
	output := flag.String("o", "simple", "Output `format`. Valid values simple, alt, flag, basic, detailed")
	assertFormat := flag.Bool("f", false, "Enable format assertions with draft >= 2019")
	assertContent := flag.Bool("c", false, "Enable content assertions with draft >= 7")
	insecure := flag.Bool("k", false, "Use insecure TLS connection")
	flag.Parse()

	// help --
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// draft --
	var draft *jsonschema.Draft
	switch *draftVersion {
	case 4:
		draft = jsonschema.Draft4
	case 6:
		draft = jsonschema.Draft6
	case 7:
		draft = jsonschema.Draft7
	case 2019:
		draft = jsonschema.Draft2019
	case 2020:
		draft = jsonschema.Draft2020
	default:
		eprintln("invalid draft: %v", *draftVersion)
		eprintln("")
		flag.Usage()
		os.Exit(2)
	}

	// output --
	if !slices.Contains([]string{"simple", "alt", "flag", "basic", "detailed"}, *output) {
		eprintln("invalid output: %v", *output)
		eprintln("")
		flag.Usage()
		os.Exit(2)
	}

	// schema --
	if len(flag.Args()) == 0 {
		eprintln("missing SCHEMA")
		eprintln("")
		flag.Usage()
		os.Exit(2)
	}
	schema := flag.Args()[0]

	// setup compiler
	c := jsonschema.NewCompiler()
	if draft != nil {
		c.DefaultDraft(draft)
	}
	if *assertFormat {
		c.AssertFormat()
	}
	if *assertContent {
		c.AssertContent()
	}
	c.UseLoader(newLoader(*insecure))

	// compile
	sch, err := c.Compile(schema)
	if err != nil {
		fmt.Printf("schema %s: failed\n", schema)
		if !*quiet {
			fmt.Println(err)
		}
		os.Exit(1)
	}
	fmt.Printf("schema %s: ok\n", schema)

	// validate
	allValid := true
	for _, instance := range flag.Args()[1:] {
		if !*quiet {
			fmt.Println()
		}
		f, err := os.Open(instance)
		if err != nil {
			fmt.Printf("instance %s: failed\n", instance)
			if !*quiet {
				fmt.Printf("error opening file %v: %v\n", instance, err)
			}
			allValid = false
			continue
		}
		defer f.Close()

		var inst any
		if ext := filepath.Ext(instance); ext == ".yaml" || ext == ".yml" {
			err = yaml.NewDecoder(f).Decode(&inst)
		} else {
			inst, err = jsonschema.UnmarshalJSON(f)
		}
		if err != nil {
			fmt.Printf("instance %s: failed\n", instance)
			if !*quiet {
				fmt.Printf("error parsing file %v: %v\n", instance, err)
			}
			allValid = false
			continue
		}

		err = sch.Validate(inst)
		if err != nil {
			fmt.Printf("instance %s: failed\n", instance)
			if !*quiet {
				if verr, ok := err.(*jsonschema.ValidationError); ok {
					switch *output {
					case "simple":
						fmt.Printf("%v\n", verr)
					case "alt":
						fmt.Printf("%#v\n", verr)
					case "flag":
						printJSON(verr.FlagOutput())
					case "basic":
						printJSON(verr.BasicOutput())
					case "detailed":
						printJSON(verr.DetailedOutput())
					}
				} else {
					fmt.Println(err)
				}
			}
			allValid = false
			continue
		}
		fmt.Printf("instance %s: ok\n", instance)
	}
	if !allValid {
		os.Exit(1)
	}
}

func eprintln(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
}

func printJSON(v any) {
	b, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}
