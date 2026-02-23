package tui

import (
	"strings"

	"github.com/luanzeba/gh-csd/internal/gh"
	"github.com/mattn/go-runewidth"
)

func renderHeader(specs []columnSpec) string {
	values := make([]string, len(specs))
	for i, spec := range specs {
		values[i] = spec.title
	}
	return renderValues(values, specs)
}

func renderRow(specs []columnSpec, cs gh.Codespace) string {
	values := make([]string, len(specs))
	for i, spec := range specs {
		values[i] = spec.value(cs)
	}
	return renderValues(values, specs)
}

func renderValues(values []string, specs []columnSpec) string {
	cells := make([]string, len(specs))
	for i, spec := range specs {
		cells[i] = renderCell(values[i], spec.width)
	}
	return strings.Join(cells, columnSeparator)
}

func renderCell(value string, width int) string {
	truncated := runewidth.Truncate(value, width, "…")
	return padRight(truncated, width)
}

func padRight(value string, width int) string {
	padding := width - runewidth.StringWidth(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}
