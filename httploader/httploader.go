// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package httploader

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/santhosh-tekuri/jsonschema/loader"
)

type httpLoader struct{}

func (httpLoader) Load(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status code %d", url, resp.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}

func init() {
	loader.Register("http", httpLoader{})
	loader.Register("https", httpLoader{})
}
