package glance

import "testing"

func TestDockerContainersRemoteSourceURL(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "IPv4 address",
			source: "tcp://127.0.0.1:2375",
			want:   "http://127.0.0.1:2375",
		},
		{
			name:   "hostname",
			source: "tcp://docker.example.com:2375",
			want:   "http://docker.example.com:2375",
		},
		{
			name:   "IPv6 address",
			source: "tcp://[::1]:2375",
			want:   "http://[::1]:2375",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dockerContainersRemoteSourceURL(tt.source)
			if err != nil {
				t.Fatalf("dockerContainersRemoteSourceURL() returned an error: %v", err)
			}

			if got != tt.want {
				t.Errorf("dockerContainersRemoteSourceURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
