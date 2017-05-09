// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package loader

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type Loader interface {
	Load(url string) ([]byte, error)
}

type filePathLoader struct{}

func (filePathLoader) Load(path string) ([]byte, error) {
	return ioutil.ReadFile(path)
}

type fileURLLoader struct{}

func (fileURLLoader) Load(url string) ([]byte, error) {
	f := strings.TrimPrefix(url, "file://")
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(f, "/") {
			f = f[1:]
		}
		f = filepath.FromSlash(f)
	}
	return ioutil.ReadFile(f)
}

var registry = make(map[string]Loader)
var mutex = sync.RWMutex{}

type SchemeNotRegisteredError string

func (s SchemeNotRegisteredError) Error() string {
	return fmt.Sprintf("no Loader registered for schema %s", s)
}

func Register(scheme string, loader Loader) {
	mutex.Lock()
	defer mutex.Unlock()
	registry[scheme] = loader
}

func UnRegister(scheme string) {
	mutex.Lock()
	defer mutex.Unlock()
	delete(registry, scheme)
}

func get(s string) (Loader, error) {
	mutex.RLock()
	defer mutex.RUnlock()
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	if loader, ok := registry[u.Scheme]; ok {
		return loader, nil
	} else {
		return nil, SchemeNotRegisteredError(u.Scheme)
	}
}

func Load(url string) ([]byte, error) {
	loader, err := get(url)
	if err != nil {
		return nil, err
	}
	return loader.Load(url)
}

func init() {
	Register("", filePathLoader{})
	Register("file", fileURLLoader{})
}
