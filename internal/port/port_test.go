package port

import (
	"reflect"
	"testing"
)

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		in   string
		host string
		port int
		ok   bool
	}{
		{"*:80", "*", 80, true},
		{"127.0.0.1:6379", "127.0.0.1", 6379, true},
		{"[::1]:5432", "::1", 5432, true},
		{"[::]:443", "*", 443, true},
		{"0.0.0.0:22", "*", 22, true},
		{"[fe80::1%lo0]:80", "fe80::1", 80, true},
		{"*:*", "", 0, false},
		{"nonsense", "", 0, false},
		{"*:70000", "", 0, false},
	}
	for _, tt := range tests {
		host, port, ok := splitHostPort(tt.in)
		if host != tt.host || port != tt.port || ok != tt.ok {
			t.Errorf("splitHostPort(%q) = %q, %d, %v; want %q, %d, %v", tt.in, host, port, ok, tt.host, tt.port, tt.ok)
		}
	}
}

func TestAnyExposed(t *testing.T) {
	tests := []struct {
		addrs []string
		want  bool
	}{
		{[]string{"127.0.0.1"}, false},
		{[]string{"127.0.0.1", "::1"}, false},
		{[]string{"*"}, true},
		{[]string{"127.0.0.1", "*"}, true},
		{[]string{"192.168.1.20"}, true},
		{[]string{"fe80::1"}, true},
	}
	for _, tt := range tests {
		if got := anyExposed(tt.addrs); got != tt.want {
			t.Errorf("anyExposed(%v) = %v; want %v", tt.addrs, got, tt.want)
		}
	}
}

func TestNormalizeMergesAndSorts(t *testing.T) {
	raw := []Listener{
		{PID: 20, Port: 8080, Proto: "tcp", Addresses: []string{"::1"}},
		{PID: 10, Port: 80, Proto: "tcp", Addresses: []string{"*"}},
		{PID: 20, Port: 8080, Proto: "tcp", Addresses: []string{"127.0.0.1"}}, // aynı süreç, IPv4 + IPv6
		{PID: 20, Port: 8080, Proto: "udp", Addresses: []string{"*"}},
		{PID: 5, Port: 8080, Proto: "tcp", Addresses: []string{"127.0.0.1"}},
	}
	got := normalize(raw)
	if len(got) != 4 {
		t.Fatalf("len = %d; want 4: %+v", len(got), got)
	}
	order := [][3]any{{80, "tcp", 10}, {8080, "tcp", 5}, {8080, "tcp", 20}, {8080, "udp", 20}}
	for i, o := range order {
		if got[i].Port != o[0] || got[i].Proto != o[1] || got[i].PID != o[2] {
			t.Errorf("got[%d] = %d/%s pid %d; want %v", i, got[i].Port, got[i].Proto, got[i].PID, o)
		}
	}
	if want := []string{"127.0.0.1", "::1"}; !reflect.DeepEqual(got[2].Addresses, want) {
		t.Errorf("merged addresses = %v; want %v", got[2].Addresses, want)
	}
	if got[2].Exposed {
		t.Error("loopback-only listener must not be exposed")
	}
	if !got[0].Exposed {
		t.Error("wildcard listener must be exposed")
	}
}

func TestParsePorts(t *testing.T) {
	got, err := ParsePorts([]string{"80,443", "3000-3002", " 5432 "})
	if err != nil {
		t.Fatal(err)
	}
	want := []PortRange{{80, 80}, {443, 443}, {3000, 3002}, {5432, 5432}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v; want %v", got, want)
	}
	for _, bad := range []string{"0", "65536", "abc", "10-5", "1-", "-5", "80-90-100"} {
		if _, err := ParsePorts([]string{bad}); err == nil {
			t.Errorf("ParsePorts(%q) should fail", bad)
		}
	}
}

func TestFilter(t *testing.T) {
	ls := []Listener{
		{PID: 1, Command: "redis-server", Args: "redis-server *:6379", Port: 6379, UID: 501, Exposed: true},
		{PID: 2, Command: "postgres", Port: 5432, UID: 501},
		{PID: 3, Command: "nginx", BrewService: "nginx", Port: 8080, UID: 0},
		{PID: 4, Command: "x.exe", Port: 9000, UID: -1},
	}
	count := func(f Filter) int { return len(f.Apply(ls)) }

	if n := count(Filter{}); n != 4 {
		t.Errorf("empty filter = %d; want 4", n)
	}
	if n := count(Filter{Ports: []PortRange{{5000, 7000}}}); n != 2 {
		t.Errorf("port range = %d; want 2", n)
	}
	if n := count(Filter{Name: "REDIS"}); n != 1 {
		t.Errorf("name = %d; want 1", n)
	}
	if n := count(Filter{Name: "nginx"}); n != 1 {
		t.Errorf("brew name = %d; want 1", n)
	}
	if n := count(Filter{ExposedOnly: true}); n != 1 {
		t.Errorf("exposed = %d; want 1", n)
	}
	// UID 501'e ait olanlar + UID'si bilinmeyen (Windows) kayıtlar
	if n := count(Filter{OnlyUID: true, UID: 501}); n != 3 {
		t.Errorf("uid = %d; want 3", n)
	}
	// Windows'ta çağıran UID -1: filtre uygulanmaz
	if n := count(Filter{OnlyUID: true, UID: -1}); n != 4 {
		t.Errorf("uid -1 = %d; want 4", n)
	}
}

func TestParseSignal(t *testing.T) {
	for in, want := range map[string]Signal{"term": SigTERM, "SIGKILL": SigKILL, "Hup": SigHUP, " int ": SigINT} {
		got, err := ParseSignal(in)
		if err != nil || got != want {
			t.Errorf("ParseSignal(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParseSignal("USR1"); err == nil {
		t.Error("USR1 should be rejected")
	}
}
