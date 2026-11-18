package main

import (
	"strings"
	"testing"
)

func TestDecodeYAMLMultiDocument(t *testing.T) {
	in := "a: abc\n---\na: de\n"
	docs, err := decodeYAML(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("got %d docs, want 2", len(docs))
	}
	got0, _ := docs[0].(map[string]any)["a"]
	got1, _ := docs[1].(map[string]any)["a"]
	if got0 != "abc" || got1 != "de" {
		t.Fatalf("docs=%#v", docs)
	}
}

func TestDecodeYAMLSingleDocument(t *testing.T) {
	docs, err := decodeYAML(strings.NewReader("a: abc\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
}
