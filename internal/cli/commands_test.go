package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseAge(t *testing.T) {
	valid := map[string]time.Duration{
		"36h": 36 * time.Hour,
		"10d": 10 * 24 * time.Hour,
		"2w":  14 * 24 * time.Hour,
	}
	for text, want := range valid {
		if got, err := parseAge(text); err != nil || got != want {
			t.Errorf("parseAge(%q) = %v, %v, want %v", text, got, err, want)
		}
	}

	for _, text := range []string{"", "d", "10", "10m", "0d", "-1d", "+1d", "1.5d", "d10", "99999999999999w"} {
		if _, err := parseAge(text); err == nil {
			t.Errorf("parseAge(%q) succeeded, want an error", text)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := map[int64]string{
		0:               "0 B",
		1023:            "1023 B",
		1536:            "1.5 KB",
		5 * 1024 * 1024: "5.0 MB",
		3 << 30:         "3.0 GB",
	}
	for size, want := range tests {
		if got := formatBytes(size); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", size, got, want)
		}
	}
}

func TestScoopTarget(t *testing.T) {
	dir := t.TempDir()
	shim := filepath.Join(dir, "tabby.exe")
	target := filepath.Join(dir, "apps", "tabby", "current", "tabby.exe")
	if err := os.WriteFile(shim, nil, 0600); err != nil {
		t.Fatal(err)
	}

	if got := scoopTarget(shim); got != shim {
		t.Fatalf("scoopTarget() without a .shim file = %q, want %q", got, shim)
	}

	content := "path = \"" + target + "\"\r\nargs = \n"
	if err := os.WriteFile(filepath.Join(dir, "tabby.shim"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if got := scoopTarget(shim); got != target {
		t.Fatalf("scoopTarget() = %q, want %q", got, target)
	}
}
