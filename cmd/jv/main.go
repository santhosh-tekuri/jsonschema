// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v4"
	_ "github.com/santhosh-tekuri/jsonschema/v4/httploader"
)

func usage() {
	fmt.Fprintln(os.Stderr, "jv [-draft INT] <json-schema> [<json-doc>]...")
	flag.PrintDefaults()
}

func main() {
	draft := flag.Int("draft", 2019, "draft used when '$schema' attribute is missing")
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
	default:
		fmt.Fprintln(os.Stderr, "draft must be 4, 5, 7 or 2019")
		os.Exit(1)
	}
	schema, err := compiler.Compile(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%#v\n", err)
		os.Exit(1)
	}

	for _, f := range flag.Args()[1:] {
		r, err := jsonschema.LoadURL(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in reading %q. reason: \n%v\n", f, err)
			os.Exit(1)
		}

		err = schema.Validate(r)
		_ = r.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%#v\n", err)
			os.Exit(1)
		}
	}
}
