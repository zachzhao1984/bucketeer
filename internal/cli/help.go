package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"
)

// parseFlagSet parses one command's arguments, taking flags before or
// after the positional arguments. The flag package alone stops at the
// first positional one, which would silently drop `--expiry` in
// `share KEY --expiry 60`.
func parseFlagSet(fs *flag.FlagSet, args []string) error {
	ordered, err := orderFlags(fs, args)
	if err != nil {
		return err
	}
	if err := fs.Parse(ordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return usagef("run bkt %s --help for the flags it takes", fs.Name())
		}
		return usagef("%v", err)
	}
	return nil
}

// orderFlags moves every flag before the positional arguments, resolving
// whether each one takes a value, and rejecting a flag the command does
// not have rather than treating it as a path.
func orderFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	flags := make([]string, 0, len(args))
	positional := make([]string, 0, len(args))

	// afterDashDash is true once -- has been seen: what follows is a path,
	// even when it starts with a dash.
	afterDashDash := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case afterDashDash:
			positional = append(positional, arg)
		case arg == "--":
			afterDashDash = true
		case arg == "-" || !isFlag(arg):
			positional = append(positional, arg)
		default:
			trimmed := strings.TrimLeft(arg, "-")
			name, inline, hasInline := strings.Cut(trimmed, "=")
			f := fs.Lookup(name)
			if f == nil {
				return nil, usagef("%s has no flag %q", fs.Name(), arg)
			}
			if takesValue(f) && !hasInline {
				if i+1 >= len(args) {
					return nil, usagef("-%s needs a value", name)
				}
				i++
				flags = append(flags, "-"+name+"="+args[i])
				continue
			}
			if hasInline {
				flags = append(flags, "-"+name+"="+inline)
				continue
			}
			flags = append(flags, "-"+name)
		}
	}
	if afterDashDash {
		return append(flags, append([]string{"--"}, positional...)...), nil
	}
	return append(flags, positional...), nil
}

// takesValue reports whether a flag needs a value beside it.
func takesValue(f *flag.Flag) bool {
	boolFlag, ok := f.Value.(interface{ IsBoolFlag() bool })
	return !ok || !boolFlag.IsBoolFlag()
}

// versionLine is what --version prints.
func versionLine(cfg Config) string {
	return fmt.Sprintf("bkt 0.1.0 (%s/%s)\n%s", runtime.GOOS, runtime.GOARCH, cfg.ExePath)
}

// topHelp is the list of commands, as the window's Settings page shows it.
func topHelp() string {
	return strings.TrimLeft(`
bkt — the Bucketeer command line, working on the connections the app saved

Usage:
  bkt [--profile NAME] [--bucket NAME] [--json] <command> [arguments]

Storage:
  ls [PREFIX] [-r] [--limit N]      list a folder, or every descendant
  find [QUERY] [PREFIX]             search names, globs, sizes and ages
        [--glob P] [--min-size S] [--max-size S]
        [--older-than D] [--newer-than D]
        [--sort size|date|name] [--limit N]
  stat KEY                          an object's size, type and metadata
  usage [PREFIX]                    objects and bytes, by kind and folder
  cat KEY [--force]                 print an object's bytes
  get SRC... DEST [-r]              download objects or a prefix tree
  put SRC... DEST [-r]              upload files, stdin or a directory
  cp SRC DST [-r]                   copy objects, on the server
  mv SRC DST [-r]                   move objects
  rm KEY... [-r]                    delete objects; -r takes a prefix
  mkdir PREFIX                      create a folder
  share KEY [--expiry SECS]         a presigned link, valid up to 7 days

Setting up:
  profiles                          list the saved connections
  skills list                       the skill this build carries
  skills print                      its full instructions
  skills entry                      the SKILL.md to install
  skills install [--dir DIR]        install it where an agent reads it
  skills export DIR                 write the bundle somewhere else

Flags before the command:
  --profile NAME                    a connection other than the first
  --bucket NAME                     a bucket other than the saved one
  --json                            print one JSON document instead of text

get, put, cp, mv and rm also take --dry-run and --no-clobber.
`, "\n")
}

// commandHelp is the short help of one command, for the window.
func commandHelp(name string) string {
	switch name {
	case "ls":
		return "bkt ls [PREFIX] [-r] [--limit N]"
	case "find":
		return "bkt find [QUERY] [PREFIX] [--glob P] [--min-size S] [--sort size|date|name]"
	case "stat":
		return "bkt stat KEY"
	case "usage":
		return "bkt usage [PREFIX]"
	case "cat":
		return "bkt cat KEY [--force]"
	case "get":
		return "bkt get SRC... DEST [-r] [--dry-run] [--no-clobber]"
	case "put":
		return "bkt put SRC... DEST [-r] [--dry-run] [--no-clobber]"
	case "cp":
		return "bkt cp SRC DST [-r] [--dry-run] [--no-clobber]"
	case "mv":
		return "bkt mv SRC DST [-r] [--dry-run] [--no-clobber]"
	case "rm":
		return "bkt rm KEY... [-r] [--dry-run]"
	case "mkdir":
		return "bkt mkdir PREFIX"
	case "share":
		return "bkt share KEY [--expiry SECS]"
	case "profiles":
		return "bkt profiles [--json]"
	}
	return "bkt " + name
}

// timeDurationSeconds turns a number of seconds into a duration.
func timeDurationSeconds(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

// timeOrZero parses a timestamp a JSON document printed.
func timeOrZero(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// discard is where a command writes when its caller asked for none.
var discard io.Writer = io.Discard
