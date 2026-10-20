package main

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"

	"bucketeer/internal/cli"
	"bucketeer/internal/store"
)

// The app is its own command line, so there is one program to install and
// nothing that can drift out of step with the window. Settings writes the
// `bkt` link that points here.
const cliLinkName = "bkt"

// runCLI executes the command line and returns the exit code.
func runCLI(args []string) int {
	dir, err := dataDirectory()
	if err != nil {
		return cli.Run(args, cli.Config{
			Stdout: os.Stdout, Stderr: os.Stderr, Stdin: os.Stdin,
			Profiles: store.NewProfileStore("", secretStore{}),
		})
	}
	return cli.Run(args, cli.Config{
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
		Stdin:    os.Stdin,
		ExePath:  exePath(),
		Profiles: store.NewProfileStore(filepath.Join(dir, "connections.json"), secretStore{}),
	})
}

// exePath is this program's own path, resolved through any link that
// started it, so an installed `bkt` link still points an agent at the app.
func exePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "bkt"
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return exe
}

// dataDirectory is where the app keeps its connections: the one the window
// reads, found even when this program is started from a link or another
// directory.
func dataDirectory() (string, error) {
	// An explicit override keeps tests and scripts off the real keychain.
	if dir := os.Getenv("BUCKETEER_DATA_DIR"); dir != "" {
		return dir, nil
	}
	// The app's own name is what MyGo joins to the configuration
	// directory, and a bundle reports it; a plain binary does not, so the
	// names the app has been packaged under are tried as well.
	candidates := []string{}
	if dir, err := mygo.App.Path(mygo.PathUserData); err == nil {
		candidates = append(candidates, dir)
	}
	if config, err := os.UserConfigDir(); err == nil {
		for _, name := range []string{appName, "Bucketeer", "Bucketeer Dev"} {
			candidates = append(candidates, filepath.Join(config, name))
		}
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "connections.json")); err == nil {
			return dir, nil
		}
	}
	if len(candidates) > 0 {
		return candidates[0], nil
	}
	return "", os.ErrNotExist
}

/* ---- The installed link ---- */

// cliLinkPath is where the app puts the `bkt` link: the first directory of
// the user's that is on PATH, or the conventional one.
func cliLinkPath() string {
	return filepath.Join(cliLinkDir(), cliLinkName)
}

// cliLinkDir picks the directory the link goes in.
func cliLinkDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/usr/local/bin"
	}
	// ~/.local/bin is where the app installs without a password, and it is
	// on PATH in most setups; /opt/homebrew/bin and /usr/local/bin take a
	// password and are left alone.
	return filepath.Join(home, ".local", "bin")
}

// cliLinkPresent reports whether the link exists and points at this app.
func cliLinkPresent() (exists bool, current bool) {
	return cliLinkPresentAt(exePath())
}

// cliLinkPresentAt reports whether the link points at program.
func cliLinkPresentAt(program string) (exists bool, current bool) {
	target, err := os.Readlink(cliLinkPath())
	if err != nil {
		if st, statErr := os.Stat(cliLinkPath()); statErr == nil && st.Mode().IsRegular() {
			return true, false
		}
		return false, false
	}
	return true, samePath(target, program)
}

// samePath reports whether two paths name the same executable.
func samePath(a, b string) bool {
	if a == b {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// installCLILink creates the `bkt` link, replacing one that points
// somewhere else.
func (s *Service) installCLILink() (string, error) {
	return installLink(exePath())
}

// installLink points the `bkt` link at program.
func installLink(program string) (string, error) {
	dir := cliLinkDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	link := cliLinkPath()
	// Remove first: a link to an older build is still a link, and
	// os.Symlink refuses to replace one.
	if st, err := os.Lstat(link); err == nil && st.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(link); err != nil {
			return "", err
		}
	}
	if err := os.Symlink(program, link); err != nil {
		return "", err
	}
	return link, nil
}

// removeCLILink deletes the `bkt` link, when it is this app's.
func (s *Service) removeCLILink() error {
	return removeLink(exePath())
}

// removeLink deletes the `bkt` link when it points at program.
func removeLink(program string) error {
	if exists, current := cliLinkPresentAt(program); !exists || !current {
		return nil
	}
	return os.Remove(cliLinkPath())
}

// onPath reports whether a directory is on the user's PATH.
func onPath(dir string) bool {
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if entry == dir {
			return true
		}
	}
	return false
}

// platformShell names the file a person edits to add a directory to PATH.
func platformShell() string {
	if runtime.GOOS == "windows" {
		return "the System settings"
	}
	return "~/.zshrc"
}
