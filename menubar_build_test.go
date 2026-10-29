package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// MyGo dev and release builds default to CGO_ENABLED=0. A successful build
// must include the real menu bar implementation, not a silent no-op fallback.
func TestMenuBarNoCgoBuildIncludesNativeImplementation(t *testing.T) {
	cmd := exec.Command("go", "list", "-json", ".")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOOS=") && !strings.HasPrefix(entry, "GOARCH=") && !strings.HasPrefix(entry, "CGO_ENABLED=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GOOS=darwin", "GOARCH=arm64", "CGO_ENABLED=0")
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct{ GoFiles []string }
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(pkg.GoFiles, "menubar_darwin.go") {
		t.Fatal("MyGo's default no-CGO build excludes the native menu bar implementation")
	}
	if slices.Contains(pkg.GoFiles, "menubar_other.go") {
		t.Fatal("macOS build includes the no-op menu bar fallback")
	}
}
