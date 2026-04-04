package cmd

import (
	"strings"
	"testing"

	"github.com/luanzeba/gh-csd/internal/config"
)

func TestBuildCreateRepoOptions(t *testing.T) {
	cfg := &config.Config{
		Repos: map[string]config.Repo{
			"github/zeta":  {Alias: "z"},
			"github/alpha": {},
			"github/beta":  {Alias: "b"},
		},
	}

	options := buildCreateRepoOptions(cfg)
	if len(options) != 4 {
		t.Fatalf("expected 4 options, got %d", len(options))
	}

	if options[0].repo != "github/alpha" {
		t.Fatalf("expected first repo to be github/alpha, got %q", options[0].repo)
	}
	if options[0].label != "-\tgithub/alpha" {
		t.Fatalf("unexpected first label: %q", options[0].label)
	}

	if options[1].repo != "github/beta" {
		t.Fatalf("expected second repo to be github/beta, got %q", options[1].repo)
	}
	if options[1].label != "b\tgithub/beta" {
		t.Fatalf("unexpected second label: %q", options[1].label)
	}

	if options[2].repo != "github/zeta" {
		t.Fatalf("expected third repo to be github/zeta, got %q", options[2].repo)
	}
	if options[2].label != "z\tgithub/zeta" {
		t.Fatalf("unexpected third label: %q", options[2].label)
	}

	last := options[len(options)-1]
	if !last.isManual {
		t.Fatalf("expected last option to be manual")
	}
	if !strings.Contains(last.label, "Enter owner/repo manually") {
		t.Fatalf("unexpected manual label: %q", last.label)
	}
}

func TestNormalizeManualRepoInput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "plain", input: "luanzeba/dotfiles", want: "luanzeba/dotfiles"},
		{name: "trim spaces", input: "  luanzeba/dotfiles  ", want: "luanzeba/dotfiles"},
		{name: "github https url", input: "https://github.com/luanzeba/dotfiles", want: "luanzeba/dotfiles"},
		{name: "github http url", input: "http://github.com/luanzeba/dotfiles", want: "luanzeba/dotfiles"},
		{name: "github url with git suffix", input: "https://github.com/luanzeba/dotfiles.git", want: "luanzeba/dotfiles"},
		{name: "missing owner", input: "dotfiles", wantErr: true},
		{name: "missing repo", input: "luanzeba/", wantErr: true},
		{name: "too many path segments", input: "luanzeba/dotfiles/extra", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeManualRepoInput(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (value=%q)", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
