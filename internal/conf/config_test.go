package conf

import "testing"

func TestResolveSDPIP(t *testing.T) {
	tests := []struct {
		name string
		cfg  Bootstrap
		want string
	}{
		{name: "explicit SDP address", cfg: Bootstrap{Media: Media{SDPIP: "192.168.1.8"}, Sip: SIP{Host: "192.168.1.4"}}, want: "192.168.1.8"},
		{name: "SIP host replaces loopback SDP", cfg: Bootstrap{Media: Media{SDPIP: "127.0.0.1"}, Sip: SIP{Host: "192.168.1.4"}}, want: "192.168.1.4"},
		{name: "no camera reachable address", cfg: Bootstrap{Media: Media{SDPIP: "127.0.0.1", IP: "localhost"}}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.ResolveSDPIP(); got != tt.want {
				t.Fatalf("ResolveSDPIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
