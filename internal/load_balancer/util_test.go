package load_balancer

import (
	"testing"
)

func TestShortProtocol(t *testing.T) {
	cases := []struct{ input, want string }{
		{"LOAD_BALANCER_PROTOCOL_TCP", "tcp"},
		{"LOAD_BALANCER_PROTOCOL_UDP", "udp"},
		{"tcp", "tcp"},
		{"", ""},
	}

	for _, c := range cases {
		if got := shortProtocol(c.input); got != c.want {
			t.Errorf("shortProtocol(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
