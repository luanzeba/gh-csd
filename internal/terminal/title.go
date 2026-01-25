// Package terminal provides terminal integration features.
package terminal

import (
	"fmt"
	"os"
	"strings"
)

// SetTabTitle sets the terminal tab title using OSC escape sequences.
// Works with Ghostty, iTerm2, and most modern terminal emulators.
func SetTabTitle(title string) {
	// OSC 0 sets both window and tab title
	// OSC 1 sets tab title only (preferred for our use case)
	// Using OSC 1 for tab title specifically
	fmt.Fprintf(os.Stdout, "\033]1;%s\007", title)
}

// SetWindowTitle sets the terminal window title.
func SetWindowTitle(title string) {
	fmt.Fprintf(os.Stdout, "\033]2;%s\007", title)
}

// FormatTitle formats a title string using the provided template.
// Supported placeholders:
//   - {repo}: repository name (e.g., "github/github")
//   - {short_repo}: short repository name (e.g., "github")
//   - {branch}: branch name
//   - {name}: codespace name
func FormatTitle(template string, repo, branch, name string) string {
	title := template

	// Extract short repo name
	shortRepo := repo
	if parts := strings.Split(repo, "/"); len(parts) > 1 {
		shortRepo = parts[len(parts)-1]
	}

	title = strings.ReplaceAll(title, "{repo}", repo)
	title = strings.ReplaceAll(title, "{short_repo}", shortRepo)
	title = strings.ReplaceAll(title, "{branch}", branch)
	title = strings.ReplaceAll(title, "{name}", name)

	return title
}

// IsGhostty returns true if we're running in Ghostty terminal.
func IsGhostty() bool {
	return os.Getenv("TERM_PROGRAM") == "ghostty" ||
		strings.HasPrefix(os.Getenv("TERM"), "xterm-ghostty")
}

// IsSupportedTerminal returns true if the terminal supports OSC escape sequences.
func IsSupportedTerminal() bool {
	termProgram := os.Getenv("TERM_PROGRAM")
	term := os.Getenv("TERM")

	supported := []string{
		"ghostty",
		"iTerm.app",
		"Apple_Terminal",
		"WezTerm",
		"Alacritty",
		"kitty",
	}

	for _, t := range supported {
		if termProgram == t || strings.Contains(term, strings.ToLower(t)) {
			return true
		}
	}

	// Most xterm-compatible terminals support OSC
	if strings.HasPrefix(term, "xterm") {
		return true
	}

	return false
}
