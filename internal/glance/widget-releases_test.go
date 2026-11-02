package glance

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReleaseRequestUnmarshalSource(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		source     releaseSource
		repository string
		prerelease bool
	}{
		{"string github", `glanceapp/glance`, releaseSourceGithub, "glanceapp/glance", false},
		{"string gitlab", `gitlab:inkscape/inkscape`, releaseSourceGitlab, "inkscape/inkscape", false},
		{"string dockerhub tag", `dockerhub:nginx:stable-alpine`, releaseSourceDockerHub, "nginx:stable-alpine", false},
		{"object github", "repository: glanceapp/glance\ninclude-prereleases: true", releaseSourceGithub, "glanceapp/glance", true},
		{"object github prefix", "repository: github:glanceapp/glance\ninclude-prereleases: true", releaseSourceGithub, "glanceapp/glance", true},
		{"object gitlab", "repository: gitlab:inkscape/inkscape", releaseSourceGitlab, "inkscape/inkscape", false},
		{"object codeberg", "repository: codeberg:redict/redict", releaseSourceCodeberg, "redict/redict", false},
		{"object dockerhub tag", "repository: dockerhub:nginx:stable-alpine", releaseSourceDockerHub, "nginx:stable-alpine", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r releaseRequest
			if err := yaml.Unmarshal([]byte(tt.yaml), &r); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if r.source != tt.source {
				t.Errorf("source = %q, want %q", r.source, tt.source)
			}

			if r.Repository != tt.repository {
				t.Errorf("repository = %q, want %q", r.Repository, tt.repository)
			}

			if r.IncludePreleases != tt.prerelease {
				t.Errorf("include-prereleases = %v, want %v", r.IncludePreleases, tt.prerelease)
			}
		})
	}
}

func TestReleaseRequestUnmarshalInvalidSource(t *testing.T) {
	for _, input := range []string{`gitea:owner/repo`, "repository: gitea:owner/repo"} {
		var r releaseRequest
		if err := yaml.Unmarshal([]byte(input), &r); err == nil {
			t.Errorf("expected an error for %q", input)
		}
	}
}
