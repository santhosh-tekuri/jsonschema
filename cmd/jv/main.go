// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v5"
	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader"
)

func usage() {
	fmt.Fprintln(os.Stderr, "jv [-draft INT] <json-schema> [<json-doc>]...")
	flag.PrintDefaults()
}

func main() {
	draft := flag.Int("draft", 2020, "draft used when '$schema' attribute is missing")
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
	schema, err := compiler.Compile(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%#v\n", err)
		os.Exit(1)
	}

	for _, f := range flag.Args()[1:] {
		f, err := os.Open(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		err = schema.Validate(f)
		if err != nil {
			if _, ok := err.(*jsonschema.ValidationError); ok {
				fmt.Fprintf(os.Stderr, "%#v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "validation failed: %v", err)
			}
			os.Exit(1)
		}
	}
}
