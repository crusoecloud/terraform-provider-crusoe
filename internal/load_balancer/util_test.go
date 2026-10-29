package load_balancer

import (
	"testing"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
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

func TestSortListenPortsAndBackends(t *testing.T) {
	items := []swagger.ListenPortAndBackend{
		{ListenPort: 443, Backends: []swagger.Backend{{Ip: "10.0.0.5", Port: 80}, {Ip: "10.0.0.1", Port: 80}}},
		{ListenPort: 80, Backends: []swagger.Backend{{Ip: "10.0.0.3", Port: 90}, {Ip: "10.0.0.3", Port: 20}}},
	}

	sortListenPortsAndBackends(items)

	if items[0].ListenPort != 80 || items[1].ListenPort != 443 {
		t.Fatalf("listen ports not sorted: %d, %d", items[0].ListenPort, items[1].ListenPort)
	}
	if items[0].Backends[0].Port != 20 {
		t.Errorf("backends not sorted by port: %d", items[0].Backends[0].Port)
	}
	if items[1].Backends[0].Ip != "10.0.0.1" {
		t.Errorf("backends not sorted by ip: %s", items[1].Backends[0].Ip)
	}
}
