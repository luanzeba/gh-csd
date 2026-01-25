package terminal

import "testing"

func TestFormatTitle(t *testing.T) {
	tests := []struct {
		name     string
		template string
		repo     string
		branch   string
		csName   string
		want     string
	}{
		{
			name:     "full format",
			template: "CS: {repo}:{branch}",
			repo:     "github/github",
			branch:   "master",
			csName:   "super-robot",
			want:     "CS: github/github:master",
		},
		{
			name:     "short repo",
			template: "CS: {short_repo}:{branch}",
			repo:     "github/github",
			branch:   "master",
			csName:   "super-robot",
			want:     "CS: github:master",
		},
		{
			name:     "name only",
			template: "{name}",
			repo:     "github/github",
			branch:   "feature-branch",
			csName:   "my-codespace",
			want:     "my-codespace",
		},
		{
			name:     "all placeholders",
			template: "{short_repo}:{branch} ({name})",
			repo:     "owner/repo-name",
			branch:   "main",
			csName:   "test-cs",
			want:     "repo-name:main (test-cs)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTitle(tt.template, tt.repo, tt.branch, tt.csName)
			if got != tt.want {
				t.Errorf("FormatTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
