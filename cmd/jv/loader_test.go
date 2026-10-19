package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDecodeYAMLTimestampsBecomeRFC3339Strings(t *testing.T) {
	in := "modified: 2024-10-11T00:00:00Z\nname: sample\n"
	v, err := decodeYAML(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("got %T, want map[string]any", v)
	}
	got, ok := m["modified"].(string)
	if !ok {
		t.Fatalf("modified type %T want string (yaml timestamps must not stay time.Time)", m["modified"])
	}
	want, err := time.Parse(time.RFC3339, "2024-10-11T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("modified %q is not RFC3339: %v", got, err)
	}
	if !parsed.Equal(want) {
		t.Fatalf("modified %q want %v", got, want)
	}
	if m["name"] != "sample" {
		t.Fatalf("name=%v", m["name"])
	}
}

func TestLoadFileYAMLTimestamps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.yaml")
	if err := os.WriteFile(path, []byte("modified: 2024-10-11T00:00:00Z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := loadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]any)
	if _, ok := m["modified"].(string); !ok {
		t.Fatalf("modified type %T want string", m["modified"])
	}
}
