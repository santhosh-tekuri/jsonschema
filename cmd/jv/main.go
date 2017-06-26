// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema"
	_ "github.com/santhosh-tekuri/jsonschema/httploader"
	"github.com/santhosh-tekuri/jsonschema/loader"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Fprintln(os.Stderr, "jv <json-schema> [<json-doc>]...")
		os.Exit(1)
	}

	schema, err := jsonschema.Compile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	for _, f := range os.Args[2:] {
		r, err := loader.Load(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in reading %q. reason: \n%v\n", f, err)
			os.Exit(1)
		}

		err = schema.Validate(r)
		_ = r.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%q does not conform to the schema specified. reason:\n%v\n", f, err)
			os.Exit(1)
		}
	}
}
