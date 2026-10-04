//go:build windows

package port

import (
	"os/exec"
)

// scan netstat ve tasklist çıktılarını kullanır. Kullanıcı, argüman ve
// çalışma dizini bilgisi Windows'ta toplanmaz.
func scan(opts ScanOptions) ([]Listener, error) {
	var ls []Listener
	collect := func(proto string, netstatProtos ...string) error {
		for i, np := range netstatProtos {
			out, err := exec.Command("netstat", "-ano", "-p", np).Output()
			if err != nil {
				if i == 0 {
					return err // IPv4 zorunlu; IPv6 yoksa sessizce geç
				}
				continue
			}
			ls = append(ls, parseNetstat(string(out), proto)...)
		}
		return nil
	}
	if err := collect("tcp", "tcp", "tcpv6"); err != nil {
		return nil, err
	}
	if opts.UDP {
		if err := collect("udp", "udp", "udpv6"); err != nil {
			return nil, err
		}
	}
	if len(ls) == 0 {
		return nil, nil
	}
	names := map[int]string{}
	if out, err := exec.Command("tasklist", "/FO", "CSV", "/NH").Output(); err == nil {
		names = parseTasklist(string(out))
	}
	for i := range ls {
		ls[i].Command = names[ls[i].PID]
		if ls[i].Command == "" {
			ls[i].Command = "-"
		}
	}
	return ls, nil
}

func zombie(int) bool { return false }
