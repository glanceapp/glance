package glance

import "testing"

func TestDockerContainerStateToStateIcon(t *testing.T) {
	tests := []struct {
		name   string
		state  string
		status string
		want   string
	}{
		{
			name:   "running container",
			state:  "running",
			status: "Up 2 hours",
			want:   dockerContainerStateIconOK,
		},
		{
			name:   "unhealthy running container",
			state:  "running",
			status: "Up 2 hours (unhealthy)",
			want:   dockerContainerStateIconWarn,
		},
		{
			name:  "unhealthy state",
			state: "unhealthy",
			want:  dockerContainerStateIconWarn,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := dockerContainerStateToStateIcon(test.state, test.status)
			if got != test.want {
				t.Fatalf("dockerContainerStateToStateIcon(%q, %q) = %q, want %q", test.state, test.status, got, test.want)
			}
		})
	}
}
