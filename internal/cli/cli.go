// Package cli is the command line side of Bucketeer: the same executable
// the window runs in, driven by its arguments.
//
// It reads the connections and the access keys the app saved, so a command
// acts on the bucket the window would show, under the same credentials,
// without a second configuration file to keep in step.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bucketeer/internal/store"
)

// Config is what the command line needs from the program that runs it.
type Config struct {
	// Stdout and Stderr receive the results and the errors. A nil stream
	// is discarded.
	Stdout io.Writer
	Stderr io.Writer
	// Stdin is the stream `put -` uploads.
	Stdin io.Reader
	// ExePath is the program's own path, which the installed skill points
	// an agent at.
	ExePath string
	// Profiles opens the saved connections.
	Profiles *store.ProfileStore
}

// Exit codes.
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

// usageError is a mistake in the arguments, which prints a message rather
// than a storage error.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func usagef(format string, a ...any) error {
	return usageError{msg: fmt.Sprintf(format, a...)}
}

// globalOpts are the flags that come before the command.
type globalOpts struct {
	profile string
	bucket  string
	json    bool
}

// Run executes one command and returns the process's exit code.
func Run(args []string, cfg Config) int {
	if cfg.Stdout == nil {
		cfg.Stdout = io.Discard
	}
	if cfg.Stderr == nil {
		cfg.Stderr = io.Discard
	}
	if cfg.Stdin == nil {
		cfg.Stdin = strings.NewReader("")
	}

	opts, rest, err := parseGlobal(args)
	if err != nil {
		return report(cfg, err)
	}
	if len(rest) == 0 {
		fmt.Fprint(cfg.Stderr, topHelp())
		return ExitUsage
	}
	cmd, rest := rest[0], rest[1:]

	switch cmd {
	case "help", "-h", "--help":
		fmt.Fprint(cfg.Stdout, topHelp())
		return ExitOK
	case "version", "-V", "--version":
		fmt.Fprintln(cfg.Stdout, versionLine(cfg))
		return ExitOK
	case "skills":
		return report(cfg, runSkills(rest, cfg, opts))
	case "profiles":
		return report(cfg, runProfiles(rest, cfg, opts))
	}

	handler, ok := commands[cmd]
	if !ok {
		return report(cfg, usagef("unknown command %q: run --help for the list", cmd))
	}

	session, cleanup, err := connect(cfg, opts)
	if err != nil {
		return report(cfg, err)
	}
	defer cleanup()
	return report(cfg, handler(rest, cfg, opts, session))
}

// command is one subcommand that acts on storage.
type command func(args []string, cfg Config, opts globalOpts, session *store.Session) error

// Commands returns the names of the storage commands, for the window's
// reference, so it cannot list one the program does not accept.
func Commands() map[string]bool {
	out := make(map[string]bool, len(commands))
	for name := range commands {
		out[name] = true
	}
	return out
}

var commands = map[string]command{
	"ls":    runLS,
	"find":  runFind,
	"stat":  runStat,
	"usage": runUsage,
	"cat":   runCat,
	"get":   runGet,
	"put":   runPut,
	"cp":    runCopy,
	"mv":    runMove,
	"rm":    runRemove,
	"mkdir": runMkdir,
	"share": runShare,
}

// report prints err and turns it into an exit code.
func report(cfg Config, err error) int {
	if err == nil {
		return ExitOK
	}
	var u usageError
	if errors.As(err, &u) {
		fmt.Fprintf(cfg.Stderr, "bkt: %s\n", u.msg)
		return ExitUsage
	}
	fmt.Fprintf(cfg.Stderr, "bkt: %s\n", err)
	return ExitError
}

// isFlag reports whether an argument is a flag rather than a value.
func isFlag(s string) bool { return len(s) > 1 && s[0] == '-' }

// parseGlobal reads the flags before the command, and returns the rest.
func parseGlobal(args []string) (globalOpts, []string, error) {
	var opts globalOpts
	for len(args) > 0 && isFlag(args[0]) {
		arg := args[0]
		name, inline, hasInline := strings.Cut(arg, "=")
		take := func() (string, error) {
			if hasInline {
				return inline, nil
			}
			if len(args) < 2 {
				return "", usagef("%s needs a value", name)
			}
			args = args[1:]
			return args[0], nil
		}
		switch name {
		case "--json":
			opts.json = true
		case "--profile":
			v, err := take()
			if err != nil {
				return opts, nil, err
			}
			opts.profile = v
		case "--bucket":
			v, err := take()
			if err != nil {
				return opts, nil, err
			}
			opts.bucket = v
		case "-h", "--help", "-V", "--version":
			return opts, []string{strings.TrimLeft(name, "-")}, nil
		default:
			return opts, nil, usagef("unknown flag %q: flags of a command go after it", arg)
		}
		args = args[1:]
	}
	return opts, args, nil
}

// connect opens the connection a command acts on: the one --profile names,
// or the first one saved.
func connect(cfg Config, opts globalOpts) (*store.Session, func(), error) {
	profiles, err := cfg.Profiles.List()
	if err != nil {
		return nil, nil, err
	}
	if len(profiles) == 0 {
		return nil, nil, errors.New("no connection is saved: add one in Bucketeer, under Settings → Connections")
	}
	p, err := chooseProfile(profiles, opts.profile)
	if err != nil {
		return nil, nil, err
	}
	if opts.bucket != "" {
		p.Bucket = opts.bucket
	}
	ctx, cancel := context.WithCancel(context.Background())
	session, err := cfg.Profiles.Open(ctx, p.ID)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	// The bucket a command acts on is the one the connection names, or the
	// one --bucket overrode it with.
	session.Profile.Bucket = p.Bucket
	return session, cancel, nil
}

// chooseProfile finds the connection --profile names, or the first saved.
func chooseProfile(profiles []store.Profile, want string) (store.Profile, error) {
	if want == "" {
		return profiles[0], nil
	}
	for _, p := range profiles {
		if p.ID == want || strings.EqualFold(p.Name, want) {
			return p, nil
		}
	}
	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return store.Profile{}, fmt.Errorf("no connection named %q; saved: %s", want, strings.Join(names, ", "))
}

/* ---- Output ---- */

// out writes a command's text, or its JSON document when --json was given.
type out struct {
	w    *bufio.Writer
	json bool
}

func newOut(cfg Config, opts globalOpts) *out {
	return &out{w: bufio.NewWriter(cfg.Stdout), json: opts.json}
}

// text prints a line of text, unless the caller asked for JSON.
func (o *out) text(format string, a ...any) {
	if o.json {
		return
	}
	fmt.Fprintf(o.w, format+"\n", a...)
}

// doc prints v as the command's JSON result.
func (o *out) doc(v any) error {
	if !o.json {
		return nil
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	o.w.Write(b)
	return o.w.WriteByte('\n')
}

func (o *out) flush() { o.w.Flush() }

/* ---- Small helpers ---- */

// bucketOf returns the bucket a command acts on.
func bucketOf(session *store.Session) (string, error) {
	if session.Profile.Bucket == "" {
		return "", usagef("this connection has no bucket: pass --bucket NAME, or set one in Bucketeer")
	}
	return session.Profile.Bucket, nil
}

// withSlash returns prefix with a trailing slash, "" for the root.
func withSlash(prefix string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return ""
	}
	return prefix + "/"
}

// sizeLabel formats a number of bytes.
func sizeLabel(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// dateLabel is a modification time as a day, or a relative span for
// something touched this week.
func dateLabel(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return t.Format("2006-01-02")
}

// localPath expands a leading ~ in a local path.
func localPath(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
		}
	}
	return p
}

// modeOf says whether a local path is a directory, and whether it exists.
func modeOf(p string) (isDir bool, exists bool) {
	st, err := os.Stat(p)
	if err != nil {
		return false, false
	}
	return st.IsDir(), true
}

// listPrefix returns every object under prefix, folder markers left out.
func listPrefix(ctx context.Context, session *store.Session, bucket, prefix string) ([]store.Entry, error) {
	listing, err := session.Search(ctx, bucket, withSlash(prefix), "", 200_000)
	if err != nil {
		return nil, err
	}
	return listing.Items, nil
}

// keyBase returns the last segment of a key or prefix.
func keyBase(key string) string {
	return path.Base(strings.TrimSuffix(key, "/"))
}

// exactKey returns key without a trailing slash.
func exactKey(key string) string { return strings.TrimSuffix(key, "/") }
