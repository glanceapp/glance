package glance

import (
	"encoding/json"
	"encoding/xml"
	"testing"
	"time"

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

func TestReleaseRequestUnmarshalUseTags(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		useTags bool
		wantErr bool
	}{
		{"default", "repository: glanceapp/glance", false, false},
		{"github", "repository: apache/cordova-android\nuse-tags: true", true, false},
		{"gitlab", "repository: gitlab:inkscape/inkscape\nuse-tags: true", true, false},
		{"codeberg", "repository: codeberg:redict/redict\nuse-tags: true", false, true},
		{"dockerhub", "repository: dockerhub:nginx\nuse-tags: true", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r releaseRequest
			err := yaml.Unmarshal([]byte(tt.yaml), &r)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if r.UseTags != tt.useTags {
				t.Errorf("use-tags = %v, want %v", r.UseTags, tt.useTags)
			}
		})
	}
}

func TestGithubTagToRelease(t *testing.T) {
	const body = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xml:lang="en-US">
  <id>tag:github.com,2008:https://github.com/apache/cordova-android/tags</id>
  <title>Tags from cordova-android</title>
  <updated>2025-05-01T10:00:00Z</updated>
  <entry>
    <id>tag:github.com,2008:Repository/1/14.0.1</id>
    <updated>2025-05-01T10:00:00Z</updated>
    <link rel="alternate" type="text/html" href="https://github.com/apache/cordova-android/releases/tag/14.0.1"/>
    <title>14.0.1</title>
  </entry>
  <entry>
    <id>tag:github.com,2008:Repository/1/14.0.0</id>
    <updated>2025-01-01T10:00:00Z</updated>
    <link rel="alternate" type="text/html" href="https://github.com/apache/cordova-android/releases/tag/14.0.0"/>
    <title>14.0.0</title>
  </entry>
</feed>`

	var feed githubTagsFeed
	if err := xml.Unmarshal([]byte(body), &feed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	release, err := githubTagToRelease("apache/cordova-android", feed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if release.Version != "v14.0.1" {
		t.Errorf("version = %q, want v14.0.1", release.Version)
	}
	if release.NotesUrl != "https://github.com/apache/cordova-android/releases/tag/14.0.1" {
		t.Errorf("notes url = %q", release.NotesUrl)
	}
	if want := time.Date(2025, 5, 1, 10, 0, 0, 0, time.UTC); !release.TimeReleased.Equal(want) {
		t.Errorf("time = %v, want %v", release.TimeReleased, want)
	}
	if release.Source != releaseSourceGithub || release.Name != "apache/cordova-android" {
		t.Errorf("unexpected source/name: %q %q", release.Source, release.Name)
	}

	if _, err := githubTagToRelease("apache/cordova-android", githubTagsFeed{}); err == nil {
		t.Error("expected an error for a repository without tags")
	}
}

func TestGitlabTagToRelease(t *testing.T) {
	const body = `[{"name":"INKSCAPE_1_4","commit":{"created_at":"2024-10-01T08:30:00.000+00:00"}}]`

	var tags []gitlabTagResponseJson
	if err := json.Unmarshal([]byte(body), &tags); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	release, err := gitlabTagToRelease("inkscape/inkscape", tags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if release.Version != "vinkscape_1_4" {
		t.Errorf("version = %q", release.Version)
	}
	if release.NotesUrl != "https://gitlab.com/inkscape/inkscape/-/tags/INKSCAPE_1_4" {
		t.Errorf("notes url = %q", release.NotesUrl)
	}
	if want := time.Date(2024, 10, 1, 8, 30, 0, 0, time.UTC); !release.TimeReleased.Equal(want) {
		t.Errorf("time = %v, want %v", release.TimeReleased, want)
	}

	if _, err := gitlabTagToRelease("inkscape/inkscape", nil); err == nil {
		t.Error("expected an error for a repository without tags")
	}
}
