package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"

	"bucketeer/internal/cli"
	"bucketeer/internal/store"
)

// The app is its own command line, so there is one program to install and
// nothing that can drift out of step with the window. Settings writes the
// `bkt` entry that reaches it.
const cliLinkName = "bkt"

// runCLI executes the command line and returns the exit code.
func runCLI(args []string) int {
	cfg := cli.Config{
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
		Stdin:    os.Stdin,
		ExePath:  exePath(),
		Profiles: nil,
	}
	if dir, err := dataDirectory(); err == nil {
		cfg.Profiles = store.NewProfileStore(filepath.Join(dir, "connections.json"), secretStore{})
	} else {
		cfg.Profiles = store.NewProfileStore("", secretStore{})
	}
	return cli.Run(args, cfg)
}

// exePath is this program's own path, resolved through any link that
// started it, so an installed `bkt` entry still points an agent at the app.
func exePath() string {
	exe, err := os.Executable()
	if err != nil {
		return cliLinkName
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

/* ---- The installed entry ---- */

// cliLinkPath is where the app puts the `bkt` entry.
func cliLinkPath() string {
	return filepath.Join(cliLinkDir(), cliEntryName())
}

// cliEntryName is the file that runs bkt: a bare name on Unix, where PATH
// resolves the symlink, and a .cmd on Windows, where PATH resolves
// PATHEXT and a symlink needs a privilege the app does not ask for.
func cliEntryName() string {
	if runtime.GOOS == "windows" {
		return cliLinkName + ".cmd"
	}
	return cliLinkName
}

// cliLinkDir picks the directory the entry goes in. Unix gets a folder on
// PATH the app can write without a password. Windows gets the folder the app
// itself is in, which the installer owns: Windows has no per-user folder on
// PATH by default, so the page names the folder and the line that adds it.
func cliLinkDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	if runtime.GOOS == "windows" {
		// The installed app lives in %LOCALAPPDATA%\Programs\<Name>; the
		// entry belongs beside it, so an update replaces both together.
		if dir, err := os.Executable(); err == nil {
			if real, err := filepath.EvalSymlinks(dir); err == nil {
				dir = real
			}
			return filepath.Dir(dir)
		}
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return filepath.Join(dir, "Programs", appName)
		}
		if home != "" {
			return filepath.Join(home, "AppData", "Local", "Programs", appName)
		}
		return "."
	}
	if home == "" {
		return "/usr/local/bin"
	}
	// ~/.local/bin is where the app installs without a password, and it is
	// on PATH in most setups; /opt/homebrew/bin and /usr/local/bin take a
	// password and are left alone.
	return filepath.Join(home, ".local", "bin")
}

// cliLinkPresent reports whether the entry exists and points at this app.
func cliLinkPresent() (exists bool, current bool) {
	return cliLinkPresentAt(exePath())
}

// cliLinkPresentAt reports whether the entry points at program.
func cliLinkPresentAt(program string) (exists bool, current bool) {
	path := cliLinkPath()
	if runtime.GOOS == "windows" {
		body, err := os.ReadFile(path)
		if err != nil {
			return false, false
		}
		return true, strings.Contains(string(body), program)
	}
	target, err := os.Readlink(path)
	if err != nil {
		if st, statErr := os.Stat(path); statErr == nil && st.Mode().IsRegular() {
			// A copy, not a link: it exists, and it is not this build.
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

// installCLILink creates the `bkt` entry, replacing one that points
// somewhere else.
func (s *Service) installCLILink() (string, error) {
	return installLink(exePath())
}

// installLink makes the `bkt` entry reach program: a symlink on Unix, and a
// small script on Windows, where creating a symlink needs a privilege the
// app does not have.
func installLink(program string) (string, error) {
	dir := cliLinkDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := cliLinkPath()
	// Remove first: an entry to an older build is still an entry, and
	// neither os.Symlink nor a write replaces one in every case.
	if _, err := os.Lstat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return "", err
		}
	}
	if runtime.GOOS == "windows" {
		body := "@echo off\r\n\"" + program + "\" %*\r\n"
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			return "", err
		}
		return path, nil
	}
	// A relative target is resolved against the link's own directory, which
	// is not the app's; the link must hold the program's full path.
	abs, err := filepath.Abs(program)
	if err != nil {
		abs = program
	}
	if err := os.Symlink(abs, path); err != nil {
		return "", err
	}
	return path, nil
}

// removeCLILink deletes the `bkt` entry, when it is this app's.
func (s *Service) removeCLILink() error {
	return removeLink(exePath())
}

// removeLink deletes the `bkt` entry when it points at program.
func removeLink(program string) error {
	if exists, current := cliLinkPresentAt(program); !exists || !current {
		return nil
	}
	return os.Remove(cliLinkPath())
}

// onPath reports whether a directory is on the user's PATH.
func onPath(dir string) bool {
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.EqualFold(filepath.Clean(entry), filepath.Clean(dir)) {
			return true
		}
	}
	return false
}

// platformShell names what a person edits to add a directory to PATH.
func platformShell() string {
	switch runtime.GOOS {
	case "windows":
		return "the environment variables in System settings"
	case "darwin":
		return "~/.zshrc"
	}
	return "~/.profile"
}

// pathHint is what adds the entry's directory to PATH: a line to paste into
// a shell's startup file on Unix, and an instruction on Windows, where PATH
// is a system setting rather than a file.
func pathHint() string {
	dir := cliLinkDir()
	if runtime.GOOS == "windows" {
		return "add " + dir + " to your Path, then open a new terminal"
	}
	shown := dir
	if home := os.Getenv("HOME"); home != "" {
		shown = strings.Replace(dir, home, "$HOME", 1)
	}
	return "export PATH=\"" + shown + ":$PATH\""
}

var errNoLink = errors.New("no bkt entry")

// checkLink reports a missing entry, for the page's message.
func checkLink() error {
	if exists, _ := cliLinkPresent(); !exists {
		return fmt.Errorf("%w at %s", errNoLink, cliLinkPath())
	}
	return nil
}
