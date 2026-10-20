package cli

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bucketeer/internal/store"
)

// runCapture runs a command with recorded streams.
func runCapture(t *testing.T, args []string, cfg Config) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	cfg.Stdout, cfg.Stderr = &out, &errb
	if cfg.Stdin == nil {
		cfg.Stdin = strings.NewReader("")
	}
	code = Run(args, cfg)
	return code, out.String(), errb.String()
}

// TestHelpListsEveryCommand checks that the help names every command the
// dispatcher accepts, so a command cannot be added without being
// documented.
func TestHelpListsEveryCommand(t *testing.T) {
	help := topHelp()
	for name := range commands {
		if !strings.Contains(help, name+" ") {
			t.Errorf("help does not mention %q", name)
		}
	}
	for _, name := range []string{"profiles", "skills"} {
		if !strings.Contains(help, name) {
			t.Errorf("help does not mention %q", name)
		}
	}
}

// TestUnknownCommandIsUsageError checks that a typo names the problem
// instead of trying to reach storage.
func TestUnknownCommandIsUsageError(t *testing.T) {
	code, _, stderr := runCapture(t, []string{"lst"}, Config{})
	if code != ExitUsage {
		t.Fatalf("code = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "lst") {
		t.Errorf("stderr = %q, want it to name the command", stderr)
	}
}

// TestFlagsAfterPositionalArgumentsAreRead is the case the flag package
// alone gets wrong: `share KEY --expiry 60` must read --expiry.
func TestFlagsAfterPositionalArgumentsAreRead(t *testing.T) {
	for _, args := range [][]string{
		{"key.txt", "--expiry", "60"},
		{"--expiry", "60", "key.txt"},
		{"--expiry=60", "key.txt"},
	} {
		fs := newTestFlagSet()
		if err := parseFlagSet(fs, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if got := fs.Lookup("expiry").Value.String(); got != "60" {
			t.Errorf("%v: expiry = %q, want 60", args, got)
		}
		if fs.NArg() != 1 || fs.Arg(0) != "key.txt" {
			t.Errorf("%v: positional = %v", args, fs.Args())
		}
	}
}

// TestUnknownFlagIsRejected checks that a flag a command does not have is
// reported, rather than read as a path.
func TestUnknownFlagIsRejected(t *testing.T) {
	fs := newTestFlagSet()
	err := parseFlagSet(fs, []string{"--nonesuch", "x"})
	if err == nil || !strings.Contains(err.Error(), "nonesuch") {
		t.Fatalf("err = %v, want it to name the flag", err)
	}
}

func newTestFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("share", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Int("expiry", 3600, "")
	return fs
}

// TestDoubleDashStopsFlagParsing checks that a key starting with a dash can
// still be named.
func TestDoubleDashStopsFlagParsing(t *testing.T) {
	fs := newTestFlagSet()
	if err := parseFlagSet(fs, []string{"--", "--expiry"}); err != nil {
		t.Fatal(err)
	}
	if fs.NArg() != 1 || fs.Arg(0) != "--expiry" {
		t.Fatalf("positional = %v", fs.Args())
	}
}

// TestGlob matches the patterns the instructions promise: * within a
// segment, ** across them, ? for one character.
func TestGlob(t *testing.T) {
	cases := []struct {
		pattern, key string
		want         bool
	}{
		{"**/*.tmp", "a/b/c.tmp", true},
		{"**/*.tmp", "c.tmp", true},
		{"*.tmp", "c.tmp", true},
		{"*.tmp", "a/c.tmp", false},
		{"a/*/c.txt", "a/b/c.txt", true},
		{"a/*/c.txt", "a/b/d/c.txt", false},
		{"a/**/c.txt", "a/b/d/c.txt", true},
		{"?.txt", "a.txt", true},
		{"?.txt", "ab.txt", false},
		{"**", "a/b/c", true},
		{"**/*.TMP", "a/b/c.tmp", false}, // case-sensitive, as documented
		{"a/**", "a", false},
		{"a/**", "a/b", true},
	}
	for _, c := range cases {
		if got := matchGlob(c.pattern, c.key); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.key, got, c.want)
		}
	}
}

// TestParseSize reads the spellings the instructions give, and documents
// the two kinds of megabyte.
func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"512", 512},
		{"1KB", 1000},
		{"100MB", 100_000_000},
		{"1GB", 1_000_000_000},
		{"1KiB", 1024},
		{"1MiB", 1 << 20},
		{"1.5MiB", 1<<20 + 1<<19},
		{"2k", 2000},
		{"10b", 10},
	}
	for _, c := range cases {
		got, err := parseSize(c.in)
		if err != nil {
			t.Errorf("parseSize(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
	for _, bad := range []string{"", "large", "-5MB"} {
		if _, err := parseSize(bad); err == nil {
			t.Errorf("parseSize(%q) accepted it", bad)
		}
	}
}

// TestParseAge reads the spans the instructions give.
func TestParseAge(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"90d", 90 * 24 * time.Hour},
		{"12h", 12 * time.Hour},
		{"30m", 30 * time.Minute},
		{"45s", 45 * time.Second},
		{"2w", 14 * 24 * time.Hour},
	}
	for _, c := range cases {
		got, err := parseAge(c.in)
		if err != nil || got != c.want {
			t.Errorf("parseAge(%q) = %v, %v; want %v", c.in, got, err, c.want)
		}
	}
	for _, bad := range []string{"90", "90y", "abc"} {
		if _, err := parseAge(bad); err == nil {
			t.Errorf("parseAge(%q) accepted it", bad)
		}
	}
}

// TestWithSlash normalizes prefixes the way every command expects.
func TestWithSlash(t *testing.T) {
	cases := map[string]string{
		"":        "",
		"/":       "",
		"media":   "media/",
		"media/":  "media/",
		"/media/": "media/",
		" a/b ":   "a/b/",
		"a/b/c/":  "a/b/c/",
	}
	for in, want := range cases {
		if got := withSlash(in); got != want {
			t.Errorf("withSlash(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestChooseProfile finds a connection by name or id, and explains a miss.
func TestChooseProfile(t *testing.T) {
	profiles := []store.Profile{
		{ID: "abc", Name: "Personal R2"},
		{ID: "def", Name: "Work"},
	}
	got, err := chooseProfile(profiles, "")
	if err != nil || got.ID != "abc" {
		t.Fatalf("default = %v, %v", got, err)
	}
	if got, err := chooseProfile(profiles, "Work"); err != nil || got.ID != "def" {
		t.Fatalf("by name = %v, %v", got, err)
	}
	if got, err := chooseProfile(profiles, "def"); err != nil || got.Name != "Work" {
		t.Fatalf("by id = %v, %v", got, err)
	}
	if _, err := chooseProfile(profiles, "Nope"); err == nil || !strings.Contains(err.Error(), "Personal R2") {
		t.Fatalf("miss = %v, want the saved names", err)
	}
}

// TestSkillsBundlePointsAtTheProgram checks that the installed instructions
// name the executable that produced them, and that the entry point is a
// skill an agent will accept.
func TestSkillsBundlePointsAtTheProgram(t *testing.T) {
	exe := "/Applications/Bucketeer.app/Contents/MacOS/Bucketeer"
	entry := skills(exe)[0].Body
	if !strings.HasPrefix(entry, "---\nname: "+SkillName+"\n") {
		t.Fatalf("entry does not open with the front matter an agent reads:\n%s", entry[:min(120, len(entry))])
	}
	if !strings.Contains(entry, "\nname: "+SkillName+"\n") {
		t.Fatal("entry has no name field")
	}
	if !strings.Contains(entry, "description: ") {
		t.Fatal("entry has no description")
	}
	if !strings.Contains(entry, exe) {
		t.Fatalf("entry does not name the executable %q", exe)
	}
	if strings.Contains(entry, "{{") {
		t.Fatal("entry has an unexpanded template")
	}
}

// TestInstallWritesAndReplacesTheBundle checks that installing puts both
// files in place, and that a second install with a new path rewrites them.
func TestInstallWritesAndReplacesTheBundle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	target := Target{Name: "Claude Code", Dir: "~/.claude/skills"}
	if err := Install(target, "/first/Bucketeer"); err != nil {
		t.Fatal(err)
	}
	entry, err := os.ReadFile(filepath.Join(target.SkillsDir(), "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entry), "/first/Bucketeer") {
		t.Fatal("entry does not name the installed path")
	}
	if _, err := os.Stat(filepath.Join(target.SkillsDir(), "reference.md")); err != nil {
		t.Fatal("reference.md was not written")
	}

	if err := Install(target, "/second/Bucketeer"); err != nil {
		t.Fatal(err)
	}
	entry, _ = os.ReadFile(filepath.Join(target.SkillsDir(), "SKILL.md"))
	if !strings.Contains(string(entry), "/second/Bucketeer") {
		t.Fatal("reinstall did not update the path")
	}

	statuses := Statuses("/second/Bucketeer")
	var found bool
	for _, s := range statuses {
		if s.Name == target.Name {
			found = true
			if !s.Installed || !s.Current {
				t.Fatalf("%s: installed=%v current=%v", s.Name, s.Installed, s.Current)
			}
		}
	}
	if !found {
		t.Fatal("the target is missing from the status list")
	}

	if err := Uninstall(target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target.SkillsDir()); !os.IsNotExist(err) {
		t.Fatal("uninstall left the folder behind")
	}
}

// TestInstallLeavesAnotherSkillAlone checks that a different skill under the
// same folder name is not overwritten.
func TestInstallLeavesAnotherSkillAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := Target{Name: "Agents", Dir: "~/.agents/skills"}
	if err := os.MkdirAll(target.SkillsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(target.SkillsDir(), "SKILL.md")
	if err := os.WriteFile(other, []byte("---\nname: someone-else\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Install(target, "/x/Bucketeer"); err == nil {
		t.Fatal("install replaced another skill")
	}
	body, _ := os.ReadFile(other)
	if !strings.Contains(string(body), "someone-else") {
		t.Fatal("the other skill was changed")
	}
}

// TestSkillsListAndEntryArePrintable checks the read-only subcommands an
// agent uses.
func TestSkillsListAndEntryArePrintable(t *testing.T) {
	code, stdout, stderr := runCapture(t, []string{"skills", "list"}, Config{ExePath: "/x/Bucketeer"})
	if code != ExitOK {
		t.Fatalf("skills list: %d %s", code, stderr)
	}
	if !strings.Contains(stdout, SkillName) {
		t.Fatalf("skills list printed %q", stdout)
	}

	code, stdout, _ = runCapture(t, []string{"skills", "entry"}, Config{ExePath: "/x/Bucketeer"})
	if code != ExitOK || !strings.Contains(stdout, "name: "+SkillName) {
		t.Fatalf("skills entry: %d", code)
	}

	code, stdout, _ = runCapture(t, []string{"skills", "print"}, Config{ExePath: "/x/Bucketeer"})
	if code != ExitOK || !strings.Contains(stdout, "reference") {
		t.Fatalf("skills print: %d", code)
	}
}

// min returns the smaller of two ints, for a truncated error message.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestNoConnectionsIsNotACrash pins the first-run behaviour every command
// needs: with nothing saved, a command explains what to do instead of
// indexing an empty list. The release workflow's smoke test depends on it.
func TestNoConnectionsIsNotACrash(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Profiles: store.NewProfileStore(filepath.Join(dir, "connections.json"), noSecrets{}),
	}

	// Reading the list is fine and empty.
	code, stdout, stderr := runCapture(t, []string{"profiles"}, cfg)
	if code != ExitOK {
		t.Fatalf("profiles exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "No connection is saved") {
		t.Fatalf("profiles printed %q", stdout)
	}
	code, stdout, _ = runCapture(t, []string{"--json", "profiles"}, cfg)
	if code != ExitOK || !strings.Contains(stdout, "[]") {
		t.Fatalf("profiles --json = %d, %q", code, stdout)
	}

	// Every command that acts on storage reports the missing connection and
	// exits with the storage error, never panicking.
	for _, args := range [][]string{
		{"ls"},
		{"find", ""},
		{"usage"},
		{"stat", "a.txt"},
		{"cat", "a.txt"},
		{"share", "a.txt"},
		{"mkdir", "new/"},
		{"rm", "a.txt"},
		{"get", "a.txt", "."},
		{"put", "a.txt", "b.txt"},
		{"cp", "a", "b"},
		{"mv", "a", "b"},
	} {
		code, _, stderr := runCapture(t, args, cfg)
		if code != ExitError {
			t.Errorf("%v exited %d, want %d", args, code, ExitError)
		}
		if strings.Contains(stderr, "panic") {
			t.Errorf("%v panicked: %s", args, stderr)
		}
		if !strings.Contains(stderr, "no connection is saved") {
			t.Errorf("%v said %q", args, stderr)
		}
	}
}

// noSecrets is a credential store that keeps nothing, for tests that never
// reach storage.
type noSecrets struct{}

func (noSecrets) Set(string, []byte) error { return nil }
func (noSecrets) Get(string) ([]byte, error) {
	return nil, errors.New("no secret")
}
func (noSecrets) Delete(string) error { return nil }
