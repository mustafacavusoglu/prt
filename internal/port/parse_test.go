package port

import (
	"reflect"
	"testing"
	"time"
)

func TestParseLsofFields(t *testing.T) {
	data := "p101\ncredis-server\nLmustafa\nu501\nf6\nPTCP\nn127.0.0.1:6379\nf7\nPTCP\nn[::1]:6379\n" +
		"p202\ncpostgres\nLpostgres\nu502\nf5\nPTCP\nn*:5432\n" +
		"p303\ncChrome Helper\nLmustafa\nu501\nf9\nPUDP\nn*:5353\nf10\nPUDP\nn192.168.1.5:5000->8.8.8.8:53\nf11\nPUDP\nn*:*\n"
	got := parseLsofFields(data)
	want := []Listener{
		{PID: 101, Command: "redis-server", User: "mustafa", UID: 501, Port: 6379, Proto: "tcp", Addresses: []string{"127.0.0.1"}},
		{PID: 101, Command: "redis-server", User: "mustafa", UID: 501, Port: 6379, Proto: "tcp", Addresses: []string{"::1"}},
		{PID: 202, Command: "postgres", User: "postgres", UID: 502, Port: 5432, Proto: "tcp", Addresses: []string{"*"}},
		{PID: 303, Command: "Chrome Helper", User: "mustafa", UID: 501, Port: 5353, Proto: "udp", Addresses: []string{"*"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestParseLsofCwd(t *testing.T) {
	got := parseLsofCwd("p101\nfcwd\nn/Users/m/proj\np202\nfcwd\nn/\n")
	want := map[int]string{101: "/Users/m/proj", 202: "/"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v; want %v", got, want)
	}
}

func TestParsePS(t *testing.T) {
	got := parsePS("  101    02:05 redis-server *:6379\n 202 3-04:05:06 /usr/bin/python3 -m http.server   8000\nbad line\n")
	if e := got[101]; e.args != "redis-server *:6379" || e.uptime != 2*time.Minute+5*time.Second {
		t.Errorf("101 = %+v", e)
	}
	if e := got[202]; e.args != "/usr/bin/python3 -m http.server   8000" ||
		e.uptime != 3*24*time.Hour+4*time.Hour+5*time.Minute+6*time.Second {
		t.Errorf("202 = %+v", e)
	}
	if len(got) != 2 {
		t.Errorf("len = %d; want 2", len(got))
	}
}

func TestParseEtime(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"00:05":      5 * time.Second,
		"12:34":      12*time.Minute + 34*time.Second,
		"1:02:03":    time.Hour + 2*time.Minute + 3*time.Second,
		"2-00:00:01": 48*time.Hour + time.Second,
		"garbage":    0,
		"1:xx":       0,
		"1:2:3:4":    0,
	} {
		if got := parseEtime(in); got != want {
			t.Errorf("parseEtime(%q) = %v; want %v", in, got, want)
		}
	}
}

func TestParseLaunchctl(t *testing.T) {
	out := "PID\tStatus\tLabel\n" +
		"812\t0\thomebrew.mxcl.redis\n" +
		"-\t0\thomebrew.mxcl.postgresql@16\n" +
		"340\t0\tcom.apple.Finder\n"
	got := parseLaunchctl(out)
	want := map[int]string{812: "redis"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v; want %v", got, want)
	}
}

func TestParseProcNet(t *testing.T) {
	tcp := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:18EB 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 11111 1 0000000000000000 100 0 0 10 0
   1: 00000000:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 22222 1 0000000000000000 100 0 0 10 0
   2: 0100007F:C350 0100007F:18EB 01 00000000:00000000 00:00000000 00000000  1000        0 33333 1 0000000000000000 100 0 0 10 0
`
	got := parseProcNet(tcp, "tcp")
	want := []procSock{
		{host: "127.0.0.1", port: 6379, proto: "tcp", uid: 1000, inode: 11111},
		{host: "*", port: 80, proto: "tcp", uid: 0, inode: 22222},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tcp got %+v\nwant %+v", got, want)
	}

	tcp6 := `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000001000000:1538 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 44444 1 0000000000000000 100 0 0 10 0
   1: 00000000000000000000000000000000:1F90 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 55555 1 0000000000000000 100 0 0 10 0
`
	got = parseProcNet(tcp6, "tcp")
	want = []procSock{
		{host: "::1", port: 5432, proto: "tcp", uid: 1000, inode: 44444},
		{host: "*", port: 8080, proto: "tcp", uid: 1000, inode: 55555},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tcp6 got %+v\nwant %+v", got, want)
	}

	udp := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode ref pointer drops
  10: 00000000:14E9 00000000:0000 07 00000000:00000000 00:00000000 00000000   100        0 66666 2 0000000000000000 0
  11: 0100007F:9C40 08080808:0035 01 00000000:00000000 00:00000000 00000000   100        0 77777 2 0000000000000000 0
`
	got = parseProcNet(udp, "udp")
	want = []procSock{{host: "*", port: 5353, proto: "udp", uid: 100, inode: 66666}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("udp got %+v\nwant %+v", got, want)
	}
}

func TestParseNetstat(t *testing.T) {
	tcp := `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:135            0.0.0.0:0              LISTENING       1032
  TCP    127.0.0.1:6379         0.0.0.0:0              LISTENING       4412
  TCP    127.0.0.1:6379         127.0.0.1:50555        ESTABLISHED     4412
  TCP    [::]:445               [::]:0                 LISTENING       4
  TCP    0.0.0.0:3000           0.0.0.0:0              DİNLENİYOR      777
`
	got := parseNetstat(tcp, "tcp")
	if len(got) != 4 {
		t.Fatalf("len = %d; want 4: %+v", len(got), got)
	}
	if got[1].PID != 4412 || got[1].Port != 6379 || got[1].Addresses[0] != "127.0.0.1" {
		t.Errorf("got[1] = %+v", got[1])
	}
	if got[2].Addresses[0] != "*" || got[2].Port != 445 {
		t.Errorf("got[2] = %+v", got[2])
	}

	udp := `
  Proto  Local Address          Foreign Address        PID
  UDP    0.0.0.0:123            *:*                    1234
  UDP    [::]:5353              *:*                    2222
`
	got = parseNetstat(udp, "udp")
	if len(got) != 2 || got[0].PID != 1234 || got[1].Port != 5353 {
		t.Errorf("udp got %+v", got)
	}
}

func TestParseTasklist(t *testing.T) {
	got := parseTasklist("\"System\",\"4\",\"Services\",\"0\",\"140 K\"\r\n\"redis-server.exe\",\"4412\",\"Console\",\"1\",\"9,000 K\"\r\n")
	want := map[int]string{4: "System", 4412: "redis-server.exe"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v; want %v", got, want)
	}
}
