package setup

import (
	"path/filepath"
	"testing"
)

func TestServiceAccountNameForDir(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{
			name: "simple repo name",
			dir:  "/home/user/my-project",
			want: "cc-my-project",
		},
		{
			name: "cloud-coop repo",
			dir:  "/home/user/cloud-coop",
			want: "cc-cloud-coop",
		},
		{
			name: "uppercase converted to lowercase",
			dir:  "/home/user/My-Repo",
			want: "cc-my-repo",
		},
		{
			name: "underscores replaced",
			dir:  "/home/user/my_project_name",
			want: "cc-my-project-name",
		},
		{
			name: "special chars replaced",
			dir:  "/home/user/my.project@v2",
			want: "cc-my-project-v2",
		},
		{
			name: "very long name truncated to 30",
			dir:  "/home/user/this-is-a-really-really-long-repository-name",
			want: "cc-this-is-a-really-really-lon",
		},
		{
			name: "short name padded",
			dir:  "/home/user/ab",
			want: "cc-ab-vm",
		},
		{
			name: "consecutive hyphens collapsed",
			dir:  "/home/user/my--bad--name",
			want: "cc-my-bad-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ServiceAccountNameForDir(tt.dir)

			// filepath.Abs may prepend cwd for relative paths; skip those.
			if !filepath.IsAbs(tt.dir) {
				t.Skipf("skipping relative path test")
			}

			if got != tt.want {
				t.Errorf("ServiceAccountNameForDir(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestServiceAccountNameForDir_TrailingHyphen(t *testing.T) {
	// Name that would end in hyphen after truncation
	got := ServiceAccountNameForDir("/home/user/a-very-long-name-ending-in-hyp")
	if got[len(got)-1] == '-' {
		t.Errorf("name should not end with hyphen: %q", got)
	}
	if len(got) < 6 || len(got) > 30 {
		t.Errorf("name length %d outside 6-30 range: %q", len(got), got)
	}
}
