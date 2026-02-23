package tui

import (
	"strings"

	"github.com/luanzeba/gh-csd/internal/gh"
)

const columnSeparator = "  "

type columnSpec struct {
	key      string
	title    string
	minWidth int
	width    int
	maxWidth int
	optional bool
	value    func(gh.Codespace) string
}

func defaultColumnSpecs() []columnSpec {
	return []columnSpec{
		{
			key:      "name",
			title:    "NAME",
			minWidth: 12,
			width:    12,
			maxWidth: 26,
			value: func(cs gh.Codespace) string {
				return fallback(cs.Name)
			},
		},
		{
			key:      "display",
			title:    "DISPLAY",
			minWidth: 12,
			width:    12,
			maxWidth: 28,
			optional: true,
			value: func(cs gh.Codespace) string {
				return fallback(cs.DisplayName)
			},
		},
		{
			key:      "repo",
			title:    "REPOSITORY",
			minWidth: 16,
			width:    16,
			maxWidth: 32,
			value: func(cs gh.Codespace) string {
				return fallback(cs.Repository)
			},
		},
		{
			key:      "branch",
			title:    "BRANCH",
			minWidth: 16,
			width:    16,
			maxWidth: 48,
			value: func(cs gh.Codespace) string {
				return fallback(cs.Branch)
			},
		},
		{
			key:      "state",
			title:    "STATE",
			minWidth: 8,
			width:    8,
			maxWidth: 12,
			value: func(cs gh.Codespace) string {
				return fallback(strings.ToUpper(cs.State))
			},
		},
		{
			key:      "created",
			title:    "CREATED",
			minWidth: 10,
			width:    10,
			maxWidth: 16,
			value: func(cs gh.Codespace) string {
				return formatRelative(cs.CreatedAt)
			},
		},
		{
			key:      "lastUsed",
			title:    "LAST USED",
			minWidth: 10,
			width:    10,
			maxWidth: 16,
			optional: true,
			value: func(cs gh.Codespace) string {
				return formatRelative(cs.LastUsedAt)
			},
		},
		{
			key:      "machine",
			title:    "MACHINE",
			minWidth: 10,
			width:    10,
			maxWidth: 20,
			value: func(cs gh.Codespace) string {
				return fallback(cs.MachineName)
			},
		},
	}
}

func columnsForWidth(width int) []columnSpec {
	if width <= 0 {
		width = 120
	}

	specs := defaultColumnSpecs()
	specs = trimColumns(specs, width)
	specs = expandColumns(specs, width)

	return specs
}

func trimColumns(specs []columnSpec, width int) []columnSpec {
	for totalWidth(specs) > width {
		idx := lastOptionalIndex(specs)
		if idx == -1 {
			break
		}
		specs = append(specs[:idx], specs[idx+1:]...)
	}

	for totalWidth(specs) > width {
		idx := maxSlackIndex(specs)
		if idx == -1 {
			break
		}
		specs[idx].width--
	}

	return specs
}

func expandColumns(specs []columnSpec, width int) []columnSpec {
	extra := width - totalWidth(specs)
	if extra <= 0 {
		return specs
	}

	growOrder := []string{"branch", "repo", "display", "name", "machine", "state", "created", "lastUsed"}
	for extra > 0 {
		grew := false
		for _, key := range growOrder {
			idx := indexByKey(specs, key)
			if idx == -1 {
				continue
			}
			if specs[idx].width < specs[idx].maxWidth {
				specs[idx].width++
				extra--
				grew = true
				if extra == 0 {
					break
				}
			}
		}
		if !grew {
			break
		}
	}

	return specs
}

func totalWidth(specs []columnSpec) int {
	if len(specs) == 0 {
		return 0
	}

	total := 0
	for _, spec := range specs {
		total += spec.width
	}
	total += (len(specs) - 1) * len(columnSeparator)
	return total
}

func lastOptionalIndex(specs []columnSpec) int {
	for i := len(specs) - 1; i >= 0; i-- {
		if specs[i].optional {
			return i
		}
	}
	return -1
}

func indexByKey(specs []columnSpec, key string) int {
	for i, spec := range specs {
		if spec.key == key {
			return i
		}
	}
	return -1
}

func maxSlackIndex(specs []columnSpec) int {
	idx := -1
	maxSlack := 0
	for i, spec := range specs {
		slack := spec.width - spec.minWidth
		if slack > maxSlack {
			idx = i
			maxSlack = slack
		}
	}
	if maxSlack == 0 {
		return -1
	}
	return idx
}
