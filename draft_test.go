package jsonschema

import (
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestDraftFromURL(t *testing.T) {
	tests := []struct {
		url     string
		version int
	}{
		{"http://json-schema.org/draft/2020-12/schema", 2020},   // http url
		{"https://json-schema.org/draft/2020-12/schema", 2020},  // https url
		{"https://json-schema.org/schema", draftLatest.version}, // latest
		{"https://json-schema.org/draft-04/schema", 4},
	}

	for _, test := range tests {
		d := draftFromURL(test.url)
		if d == nil {
			t.Fatalf("draft for %q not found", test.url)
		}
		if d.version != test.version {
			t.Fatalf("version for %q: got %d, want %d", test.url, d.version, test.version)
		}
	}
}

func TestDraft_collectIds(t *testing.T) {
	u := url("http://a.com/schema.json")

	doc, err := UnmarshalJSON(strings.NewReader(`{
		"id": "http://a.com/schemas/schema.json",
		"definitions": {
			"s1": { "id": "http://a.com/definitions/s1" },
			"s2": {
				"id": "../s2",
				"items": [
					{ "id": "http://c.com/item" },
					{ "id": "http://d.com/item" }
				]
			},
			"s3": {
				"definitions": {
					"s1": {
						"id": "s3",
						"items": {
							"id": "http://b.com/item"
						}
					}
				}
			},
			"s4": { "id": "http://e.com/def#abcd" }
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"":                                     "http://a.com/schemas/schema.json", // root with id
		"/definitions/s1":                      "http://a.com/definitions/s1",
		"/definitions/s2":                      "http://a.com/s2", // relative id
		"/definitions/s3/definitions/s1":       "http://a.com/schemas/s3",
		"/definitions/s3/definitions/s1/items": "http://b.com/item",
		"/definitions/s2/items/0":              "http://c.com/item",
		"/definitions/s2/items/1":              "http://d.com/item",
		"/definitions/s4":                      "http://e.com/def", // id with fragments
	}

	rr := newRoots()
	r := root{
		url:                 url(u),
		doc:                 doc,
		resources:           map[jsonPointer]*resource{},
		subschemasProcessed: map[jsonPointer]struct{}{},
	}
	if err := rr.collectResources(&r, doc, u, jsonPointer(""), dialect{Draft4, nil}); err != nil {
		t.Fatal(err)
	}

	resources := r.resources
	got := make(map[string]string)
	for ptr, res := range resources {
		got[string(ptr)] = res.id.String()
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf(" got:\n%v\nwant:\n%v", got, want)
	}
}

func TestDraft_collectAnchors(t *testing.T) {
	u := url("http://a.com/schema.json")

	doc, err := UnmarshalJSON(strings.NewReader(`{
		"$defs": {
			"s2": {
				"$id": "http://b.com",
				"$anchor": "b1", 
				"items": [
					{ "$anchor": "b2" },
					{
						"$id": "http//c.com",
						"items": [
							{"$anchor": "c1"},
							{"$dynamicAnchor": "c2"}
						]
					},
					{ "$dynamicAnchor": "b3" }
				]
			}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	rr := newRoots()
	r := root{
		url:                 url(u),
		doc:                 doc,
		resources:           map[jsonPointer]*resource{},
		subschemasProcessed: map[jsonPointer]struct{}{},
	}
	if err := rr.collectResources(&r, doc, u, jsonPointer(""), dialect{Draft2020, nil}); err != nil {
		t.Fatal(err)
	}

	resources := r.resources
	res, ok := resources[""]
	if !ok {
		t.Fatal("root resource is not collected")
	}
	if len(res.anchors) != 0 {
		t.Fatal("root resource should have no anchors")
	}

	res, ok = resources["/$defs/s2"]
	if !ok {
		t.Fatal("resource /$defs/s2 is not collected")
	}
	want := map[anchor]jsonPointer{
		"b1": "/$defs/s2",
		"b2": "/$defs/s2/items/0",
		"b3": "/$defs/s2/items/2",
	}
	if !reflect.DeepEqual(res.anchors, want) {
		t.Fatalf("anchors for /$defs/s2:\n got: %v\nwant: %v", res.anchors, want)
	}

	res, ok = resources["/$defs/s2/items/1"]
	if !ok {
		t.Fatal("resource /$defs/s2/items/1 is not collected")
	}
	want = map[anchor]jsonPointer{
		"c1": "/$defs/s2/items/1/items/0",
		"c2": "/$defs/s2/items/1/items/1",
	}
	if !reflect.DeepEqual(res.anchors, want) {
		t.Fatalf("anchors for /$defs/s2/items/1:\n got: %v\nwant: %v", res.anchors, want)
	}
}

// TestDraftMetaCompiledOnFirstUse runs in a fresh process, where no draft
// metaschema has been compiled yet: compiling a 2020-12 schema from several
// goroutines must compile only draft 2020-12's metaschemas.
func TestDraftMetaCompiledOnFirstUse(t *testing.T) {
	if os.Getenv("JSONSCHEMA_TEST_FRESH_PROCESS") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestDraftMetaCompiledOnFirstUse$", "-test.count=1")
		cmd.Env = append(os.Environ(), "JSONSCHEMA_TEST_FRESH_PROCESS=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}

	drafts := []*Draft{Draft4, Draft6, Draft7, Draft2019, Draft2020}
	for _, d := range drafts {
		if d.sch != nil {
			t.Fatalf("%s metaschema compiled before first use", d)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := NewCompiler()
			sch := map[string]any{"$schema": Draft2020.url, "type": "string"}
			if err := c.AddResource("schema.json", sch); err != nil {
				t.Error(err)
				return
			}
			if _, err := c.Compile("schema.json"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	for _, d := range drafts {
		if compiled := d.sch != nil; compiled != (d == Draft2020) {
			t.Errorf("%s metaschema compiled = %v", d, compiled)
		}
	}
	if len(Draft2020.vocabSchema) != len(Draft2020.allVocabs) {
		t.Errorf("got %d vocab metaschemas, want %d", len(Draft2020.vocabSchema), len(Draft2020.allVocabs))
	}
}
