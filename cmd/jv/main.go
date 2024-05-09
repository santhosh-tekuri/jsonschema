package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	flag "github.com/spf13/pflag"
)

func main() {
	flag.Usage = func() {
		eprintln("Usage: jv [OPTIONS] SCHEMA [INSTANCE...]")
		eprintln("")
		eprintln("Options:")
		flag.PrintDefaults()
	}
	help := flag.BoolP("help", "h", false, "Print help information")
	version := flag.BoolP("version", "v", false, "Print build information")
	quiet := flag.BoolP("quiet", "q", false, "Do not print errors")
	draftVersion := flag.IntP("draft", "d", 2020, "Draft `version` used when '$schema' is missing. Valid values 4, 6, 7, 2019, 2020")
	output := flag.StringP("output", "o", "simple", "Output `format`. Valid values simple, alt, flag, basic, detailed")
	assertFormat := flag.BoolP("assert-format", "f", false, "Enable format assertions with draft >= 2019")
	assertContent := flag.BoolP("assert-content", "c", false, "Enable content assertions with draft >= 7")
	insecure := flag.BoolP("insecure", "k", false, "Use insecure TLS connection")
	cacert := flag.String("cacert", "", "Use the specified `pem-file` to verify the peer. The file may contain multiple CA certificates")
	maps := flag.StringArrayP("map", "m", nil, "load url with prefix from given directory. Syntax `url_prefix=/path/to/dir`")
	flag.CommandLine.SortFlags = false
	flag.Parse()

	// help --
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *version {
		bi, ok := debug.ReadBuildInfo()
		if ok {
			fmt.Println(bi.Main.Path, bi.Main.Version)
			for _, dep := range bi.Deps {
				fmt.Println(dep.Path, dep.Version)
			}
		} else {
			fmt.Println("no build information available")
		}
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

	// maps --
	mappings, err := func() (map[string]string, error) {
		mappings := map[string]string{}
		for _, m := range *maps {
			equal := strings.IndexByte(m, '=')
			if equal == -1 {
				return nil, fmt.Errorf("invalid map: %v", m)
			}
			u, dir := m[:equal], m[equal+1:]
			if dir == "" {
				return nil, fmt.Errorf("invalid map: %v", m)
			}
			_, err := url.Parse(u)
			if err != nil {
				return nil, fmt.Errorf("invalid map %v: %v", m, err)
			}
			if !strings.HasSuffix(u, "/") {
				u += "/"
			}
			mappings[u] = dir
		}
		return mappings, nil
	}()
	if err != nil {
		eprintln("%v", err)
		eprintln("")
		flag.Usage()
		os.Exit(2)
	}

	stdinDecoder := json.NewDecoder(os.Stdin)
	stdinDecoder.UseNumber()

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
	loader, err := newLoader(mappings, *insecure, *cacert)
	if err != nil {
		eprintln("%v", err)
		os.Exit(2)
	}
	c.UseLoader(loader)

	// compile
	sch, err := func() (*jsonschema.Schema, error) {
		if schema == "-" {
			var v any
			if err := stdinDecoder.Decode(&v); err != nil {
				return nil, err
			}
			if err := c.AddResource("stdin.json", v); err != nil {
				return nil, err
			}
			return c.Compile("stdin.json")
		}
		return c.Compile(schema)
	}()
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
		inst, err := func() (any, error) {
			if instance == "-" {
				var inst any
				err := stdinDecoder.Decode(&inst)
				return inst, err
			}
			return loadFile(instance)
		}()
		if err != nil {
			fmt.Printf("instance %s: failed\n", instance)
			if !*quiet {
				fmt.Println(err)
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
