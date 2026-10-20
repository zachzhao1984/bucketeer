package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkillName is the skill the app installs, and the folder it lives in.
const SkillName = "bucketeer"

// skill is one file of the bundle.
type skill struct {
	// Path is the file's path inside the skill's folder.
	Path string
	// Body is the file's contents.
	Body string
}

// skills is what the app writes into an agent's skills directory: the
// entry point, plus the reference the instructions point at.
func skills(exePath string) []skill {
	return []skill{
		{Path: "SKILL.md", Body: entrySkill(exePath)},
		{Path: "reference.md", Body: referenceDoc()},
	}
}

// entrySkill is the SKILL.md: the front matter an agent reads to decide
// whether the skill is for its task, then the way to use it.
func entrySkill(exePath string) string {
	return fmt.Sprintf(`---
name: bucketeer
description: Work with the user's S3-compatible storage through the Bucketeer app on this machine — list, search, inspect, upload, download, copy, move, rename, delete, size up a bucket, and mint presigned share links. Use when asked to browse or clean up cloud storage, upload a site's assets, or find what takes up space in a bucket.
---

# Bucketeer

Bucketeer keeps the user's bucket connections, and their access keys in the
system credential store. Its command line speaks to the same connections, so
there is nothing to configure and no key to copy.

The executable below is the app itself. Always pass a subcommand: with no
arguments it opens the window instead of printing anything.

`+"```"+`
%s
`+"```"+`

## Before the first command

1. Run `+"`%s profiles`"+` to see which connection is active and its bucket.
   Quote the path literally: it has spaces.
2. Read `+"`reference.md`"+` beside this file for the exact flags of each
   command, its output shape under `+"`--json`"+`, and the safety rules.

If the executable is not there, the app was moved or removed. Stop and ask
the user; do not guess another path.

## How to work

- Prefer `+"`--json`"+` and read the fields; text output is for people.
- Find the exact key before acting on it: `+"`ls`"+` for a folder,
  `+"`find`"+` for a name, glob, size or age.
- Preview anything that writes or deletes with `+"`--dry-run`"+`, show the
  plan, and only then run it without the flag.
- Deletion is permanent. A prefix delete takes everything under it, and
  `+"`rm`"+` without `+"`-r`"+` will refuse a prefix rather than guess.
- Never print a share URL into a log or a commit. It is a bearer
  credential: anyone holding it can read the object until it expires.
- Use the app's own batch flags instead of shell loops, and never write a
  script that deletes in a loop.

## Commands at a glance

`+"```"+`
ls [PREFIX] [-r] [--limit N]      find [QUERY] [PREFIX] [--glob P]
stat KEY                          usage [PREFIX]
cat KEY [--force]                 share KEY [--expiry SECS]
get SRC... DEST [-r]              put SRC... DEST [-r]
cp SRC DST [-r]                   mv SRC DST [-r]
rm KEY... [-r]                    mkdir PREFIX
`+"```"+`

Every command takes `+"`--profile NAME`"+` and `+"`--bucket NAME`"+` before it
to act on another connection or bucket than the active one, and
`+"`--json`"+` to print one JSON document.
`, exePath, exePath)
}

// referenceDoc is the long form: one section per command.
func referenceDoc() string {
	return `# Bucketeer command reference

Every command takes these flags before it:

- ` + "`--profile NAME`" + ` — a connection other than the active one, by name or id.
- ` + "`--bucket NAME`" + ` — a bucket other than the connection's own.
- ` + "`--json`" + ` — print one JSON document instead of text.

` + "`get`" + `, ` + "`put`" + `, ` + "`cp`" + `, ` + "`mv`" + ` and ` + "`rm`" + ` take
` + "`--dry-run`" + `, which prints what would happen and touches nothing, and
` + "`--no-clobber`" + `, which skips what already exists.

Prefixes end in a slash. Keys do not.

## ls [PREFIX] [-r] [--limit N]

Lists one folder: its sub-folders, then its objects, each with a size.
` + "`-r`" + ` lists every descendant as a flat list of keys.
` + "`--limit N`" + ` stops early and sets ` + "`truncated`" + `.

` + "```json" + `
{"prefix":"media/","recursive":false,"bucket":"assets","count":2,"truncated":false,
 "entries":[{"key":"media/2024/","name":"2024","type":"folder","size":0},
            {"key":"media/logo.png","name":"logo.png","type":"object","size":2048,
             "lastModified":"2026-03-01T10:00:00Z"}]}
` + "```" + `

## find [QUERY] [PREFIX] [--glob P] [flags]

Searches below PREFIX. QUERY matches the object's name, case-insensitively;
an empty QUERY matches everything. ` + "`--glob`" + ` matches the whole key,
case-sensitively, with ` + "`*`" + ` (not across a slash), ` + "`?`" + ` and
` + "`**`" + ` (across slashes); brackets and braces are literal.

- ` + "`--min-size`" + `, ` + "`--max-size`" + ` — ` + "`100MB`" + ` is 100,000,000 bytes;
  ` + "`100MiB`" + ` is 104,857,600. Both bounds are inclusive.
- ` + "`--older-than`" + `, ` + "`--newer-than`" + ` — ` + "`90d`" + `, ` + "`12h`" + `,
  ` + "`30m`" + `, ` + "`45s`" + `, ` + "`2w`" + `.
- ` + "`--sort size|date|name`" + ` — size and date are newest or biggest first;
  name is ascending. ` + "`--limit N`" + ` defaults to 100.

Read ` + "`matches[].key`" + ` from the JSON and use those keys verbatim. Globs
are case-sensitive: for a case-insensitive sweep, run one glob per spelling.

## stat KEY

One object's size, content type, ETag, modification time, storage class and
metadata. Use it to confirm a key exists before a write.

## usage [PREFIX]

` + "`totalObjects`" + `, ` + "`totalBytes`" + `, then ` + "`bytesByKind`" + ` and
` + "`bytesByFolder`" + `, and the ten largest objects. This walks everything under
the prefix, so on a very large bucket prefer a narrower prefix.

## cat KEY [--force]

Writes the object's bytes to stdout. Refuses anything over 64 MiB unless
` + "`--force`" + ` is given. For binary data, download with ` + "`get`" + ` instead
of printing it.

## get SRC... DEST [-r]

Downloads. A destination that exists as a folder, ends in a slash, or comes
with several sources receives the objects under their own names. With
` + "`-r`" + ` and a prefix source, the tree lands under DEST with its relative
paths kept; an object source needs no ` + "`-r`" + `.

## put SRC... DEST [-r]

Uploads. A directory needs ` + "`-r`" + `; its tree lands under DEST with the
directory's own name as the first segment. ` + "`put - KEY`" + ` reads stdin and
needs the full key. The content type comes from the extension unless
` + "`--content-type`" + ` names one.

Check the plan's destination before uploading a directory: ` + "`put -r ./photos media/`" + `
writes ` + "`media/photos/...`" + `, which is usually what is wanted.

## cp SRC DST [-r], mv SRC DST [-r]

` + "`cp`" + ` copies on the server, without downloading. ` + "`mv`" + ` copies and
then deletes the source. Both take one key, or with ` + "`-r`" + ` a whole prefix,
whose relative paths are kept. Neither can move a prefix into itself.

## rm KEY... [-r] [--dry-run]

Deletes objects. A prefix needs ` + "`-r`" + `, and then takes everything under
it. The whole bucket needs ` + "`--yes-really-delete-everything`" + `.

Pass keys as separate arguments, each quoted, and preview with
` + "`--dry-run`" + ` first. Deleting a prefix that holds nothing succeeds and
deletes nothing.

## mkdir PREFIX

Creates a folder. Object storage has no directories: this writes the empty
marker object that makes one appear.

## share KEY [--expiry SECS]

A presigned GET link, valid for 1 to 604800 seconds (7 days). The link does
not check that the object exists: ` + "`stat`" + ` first when that matters. Never
log the URL or commit it.

## profiles

The saved connections, marked with the active one. Under ` + "`--json`" + ` it
prints ids, names, providers, endpoints, regions and buckets — never an
access key.

## Exit codes

` + "`0`" + ` succeeded, ` + "`1`" + ` the storage or the system refused, ` + "`2`" + ` the
arguments were wrong. The message on stderr says which.
`
}

/* ---- Where a skill can be installed ---- */

// Target is a place an agent reads skills from.
type Target struct {
	// Name is what the window calls it: "Claude Code", "Codex", "Agents".
	Name string
	// Dir is the skills directory, with ~ unexpanded.
	Dir string
}

// Targets are the skill directories the app knows about, in the order the
// Settings page lists them. Each is installed only when its parent exists,
// so a person sees the tools they actually have.
var Targets = []Target{
	{Name: "Claude Code", Dir: "~/.claude/skills"},
	{Name: "Codex", Dir: "~/.codex/skills"},
	{Name: "Agents", Dir: "~/.agents/skills"},
	{Name: "Cursor", Dir: "~/.cursor/skills"},
}

// TargetStatus is one target and whether it is there.
type TargetStatus struct {
	Target
	// Available is true when the agent's own folder exists.
	Available bool
	// Installed is true when this skill is in its skills directory, and
	// Current when that copy matches this build.
	Installed bool
	Current   bool
	// Path is where the skill is, or would be.
	Path string
}

// EntryMarkdown is the SKILL.md this build would install, for the window to
// show or copy.
func EntryMarkdown(exePath string) string { return skills(exePath)[0].Body }

// SkillsDir is where the skill goes for a target.
func (t Target) SkillsDir() string {
	dir := t.Dir
	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, strings.TrimPrefix(dir, "~/"))
		}
	}
	return filepath.Join(dir, SkillName)
}

// InstalledPath is the bundle's marker file for a target.
func (t Target) InstalledPath() string {
	return filepath.Join(t.SkillsDir(), "SKILL.md")
}

// Statuses reports every target, and whether this build is installed.
func Statuses(exePath string) []TargetStatus {
	want := skills(exePath)
	out := make([]TargetStatus, 0, len(Targets))
	for _, t := range Targets {
		parent := filepath.Dir(strings.Replace(t.Dir, "~/", "", 1))
		_ = parent
		status := TargetStatus{Target: t, Path: t.InstalledPath()}
		status.Available = agentFolderExists(t)
		body, err := os.ReadFile(t.InstalledPath())
		if err == nil {
			status.Installed = true
			status.Current = string(body) == want[0].Body
		}
		out = append(out, status)
	}
	return out
}

// agentFolderExists reports whether the agent's own directory is there,
// which is how the app knows the person uses it.
func agentFolderExists(t Target) bool {
	dir := expandHome(t.Dir)
	if _, err := os.Stat(dir); err == nil {
		return true
	}
	// A skills folder that does not exist yet still counts when the agent's
	// configuration directory does.
	parent := filepath.Dir(dir)
	if _, err := os.Stat(parent); err == nil {
		return true
	}
	return false
}

// Install writes the bundle into a target's skills directory, replacing
// what was there.
func Install(t Target, exePath string) error {
	dir := t.SkillsDir()
	// Only replace a folder that holds this skill: a different one under
	// the same name is not ours to delete.
	if existing, err := os.ReadFile(filepath.Join(dir, "SKILL.md")); err == nil {
		if !strings.Contains(string(existing), "name: "+SkillName) {
			return fmt.Errorf("%s holds another skill; move it away first", dir)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, file := range skills(exePath) {
		path := filepath.Join(dir, file.Path)
		if err := os.WriteFile(path, []byte(file.Body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Uninstall removes the bundle from a target.
func Uninstall(t Target) error {
	dir := t.SkillsDir()
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return os.RemoveAll(dir)
}

// Export writes the bundle to an arbitrary directory, for a tool the app
// does not know.
func Export(dir, exePath string) ([]string, error) {
	dir = expandHome(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var written []string
	for _, file := range skills(exePath) {
		path := filepath.Join(dir, file.Path)
		if err := os.WriteFile(path, []byte(file.Body), 0o644); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

// expandHome replaces a leading ~ with the home directory.
func expandHome(p string) string {
	if !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~/"))
}

/* ---- The skills command ---- */

// runSkills lists, prints, installs and exports the bundle.
func runSkills(args []string, cfg Config, opts globalOpts) error {
	sub := "list"
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	switch sub {
	case "list":
		return skillsList(cfg, opts)
	case "print":
		fmt.Fprint(cfg.Stdout, skills(cfg.ExePath)[1].Body)
		return nil
	case "entry":
		fmt.Fprint(cfg.Stdout, skills(cfg.ExePath)[0].Body)
		return nil
	case "install":
		fs := flag.NewFlagSet("skills install", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		dir := fs.String("dir", "", "install into this skills directory instead")
		if err := parseFlagSet(fs, args); err != nil {
			return err
		}
		if *dir != "" {
			written, err := Export(*dir, cfg.ExePath)
			if err != nil {
				return err
			}
			for _, path := range written {
				fmt.Fprintln(cfg.Stdout, path)
			}
			return nil
		}
		installed := 0
		for _, status := range Statuses(cfg.ExePath) {
			if !status.Available || status.Current {
				continue
			}
			if err := Install(status.Target, cfg.ExePath); err != nil {
				return err
			}
			fmt.Fprintf(cfg.Stdout, "Installed for %s: %s\n", status.Name, status.SkillsDir())
			installed++
		}
		if installed == 0 {
			fmt.Fprintln(cfg.Stdout, "Nothing to do: no agent folder was found, or every copy is current.")
		}
		return nil
	case "export":
		if len(args) != 1 {
			return usagef("skills export takes one directory")
		}
		written, err := Export(args[0], cfg.ExePath)
		for _, path := range written {
			fmt.Fprintln(cfg.Stdout, path)
		}
		return err
	}
	return usagef("skills takes list, print, entry, install or export")
}

// skillsList prints the skill this build carries.
func skillsList(cfg Config, opts globalOpts) error {
	dir := filepath.Dir(cfg.ExePath)
	o := newOut(cfg, opts)
	defer o.flush()
	doc := []map[string]string{{
		"name":        SkillName,
		"description": "Work with the user's S3-compatible storage through the Bucketeer app on this machine.",
		"path":        skillPath(dir),
	}}
	if err := o.doc(doc); err != nil {
		return err
	}
	o.text("%s  %s", SkillName, doc[0]["description"])
	return nil
}

// skillPath is where the bundle would be installed for a given program.
func skillPath(dir string) string {
	names := make([]string, 0, len(Targets))
	for _, t := range Targets {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
