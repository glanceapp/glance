package glance

import (
	"net/http"
	"testing"
)

func TestAddressOfRequest(t *testing.T) {
	tests := []struct {
		name       string
		proxied    bool
		remoteAddr string
		xff        string
		want       string
	}{
		{"not proxied - uses remote addr", false, "1.2.3.4:5678", "", "1.2.3.4"},
		{"proxied - no XFF header", true, "1.2.3.4:5678", "", "1.2.3.4"},
		{"proxied - single IP", true, "1.2.3.4:5678", "5.6.7.8", "5.6.7.8"},
		{"proxied - multiple IPs returns last", true, "1.2.3.4:5678", "1.2.3.4, 5.6.7.8", "5.6.7.8"},
		{"proxied - spoofed first IP ignored", true, "1.2.3.4:5678", "99.99.99.99, 1.2.3.4", "1.2.3.4"},
		{"proxied - empty XFF", true, "1.2.3.4:5678", "", "1.2.3.4"},
		{"proxied - trailing comma falls back", true, "1.2.3.4:5678", "5.6.7.8,", "1.2.3.4"},
		{"proxied - whitespace trimmed", true, "1.2.3.4:5678", "  5.6.7.8  ", "5.6.7.8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{Config: config{}}
			app.Config.Server.Proxied = tt.proxied

			req := &http.Request{RemoteAddr: tt.remoteAddr}
			if tt.xff != "" {
				req.Header = http.Header{}
				req.Header.Set("X-Forwarded-For", tt.xff)
			}

			got := app.addressOfRequest(req)
			if got != tt.want {
				t.Errorf("addressOfRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}