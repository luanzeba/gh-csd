package tui

import (
	"fmt"
	"time"
)

func formatRelative(t time.Time) string {
	if t.IsZero() {
		return "—"
	}

	if time.Now().Before(t) {
		return "just now"
	}

	age := time.Since(t)
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	case age < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(age.Hours()/24))
	case age < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(age.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(age.Hours()/(24*365)))
	}
}

func fallback(value string) string {
	if value == "" {
		return "—"
	}
	return value
}
