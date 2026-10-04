// Package port, sistemde dinlenen TCP/UDP portlarını ve bu portları açan
// süreçleri bulmak ve kapatmak için platformdan bağımsız bir API sunar.
package port

import (
	"errors"
	"net"
	"sort"
	"strings"
)

// Listener, bir süreç tarafından dinlenen tek bir (süreç, port, protokol)
// üçlüsünü temsil eder. Aynı süreç hem IPv4 hem IPv6 üzerinde dinliyorsa
// adresler tek kayıtta birleştirilir.
type Listener struct {
	PID         int      `json:"pid"`
	Command     string   `json:"process"`
	Args        string   `json:"args,omitempty"`
	User        string   `json:"user"`
	UID         int      `json:"uid"`
	Port        int      `json:"port"`
	Proto       string   `json:"proto"`
	Addresses   []string `json:"addresses"`
	Exposed     bool     `json:"exposed"`
	BrewService string   `json:"brew_service,omitempty"`
	Cwd         string   `json:"cwd,omitempty"`
	UptimeSec   int64    `json:"uptime_seconds,omitempty"`
}

// ScanOptions, Scan'in neyi toplayacağını belirler.
type ScanOptions struct {
	// UDP true ise TCP'ye ek olarak bağlanmamış (unconnected) UDP soketleri de listelenir.
	UDP bool
	// Details true ise çalışma dizini gibi maliyetli bilgiler de toplanır.
	Details bool
}

var (
	// ErrNoProcess, hedef sürecin artık var olmadığını belirtir.
	ErrNoProcess = errors.New("süreç bulunamadı")
	// ErrNotSupported, işlemin bu platformda desteklenmediğini belirtir.
	ErrNotSupported = errors.New("bu platformda desteklenmiyor")
)

// Scan dinleyen portları döndürür; sonuç port, protokol ve PID'ye göre sıralıdır.
func Scan(opts ScanOptions) ([]Listener, error) {
	raw, err := scan(opts)
	if err != nil {
		return nil, err
	}
	return normalize(raw), nil
}

// StillListening, l'deki PID'nin hâlâ aynı portu dinleyip dinlemediğini söyler.
// Onay sorulduktan sonra PID yeniden kullanılmış olabilir; kill öncesi kontrol içindir.
func StillListening(l Listener) bool {
	cur, err := Scan(ScanOptions{UDP: l.Proto == "udp"})
	if err != nil {
		return false
	}
	for _, c := range cur {
		if c.PID == l.PID && c.Port == l.Port && c.Proto == l.Proto {
			return true
		}
	}
	return false
}

func normalize(raw []Listener) []Listener {
	type key struct {
		pid, port int
		proto     string
	}
	idx := make(map[key]int, len(raw))
	var out []Listener
	for _, l := range raw {
		k := key{l.PID, l.Port, l.Proto}
		i, ok := idx[k]
		if !ok {
			idx[k] = len(out)
			l.Addresses = append([]string(nil), l.Addresses...)
			out = append(out, l)
			continue
		}
		out[i].Addresses = append(out[i].Addresses, l.Addresses...)
	}
	for i := range out {
		out[i].Addresses = uniqueSorted(out[i].Addresses)
		out[i].Exposed = anyExposed(out[i].Addresses)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Port != b.Port {
			return a.Port < b.Port
		}
		if a.Proto != b.Proto {
			return a.Proto < b.Proto
		}
		return a.PID < b.PID
	})
	return out
}

func uniqueSorted(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// WildcardAddr, "tüm arayüzler" anlamına gelen normalleştirilmiş adrestir.
const WildcardAddr = "*"

// normalizeHost joker adresleri ("0.0.0.0", "::", "*") tek biçime indirger.
func normalizeHost(host string) string {
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if i := strings.IndexByte(host, '%'); i >= 0 { // IPv6 zone: fe80::1%lo0
		host = host[:i]
	}
	switch host {
	case "*", "0.0.0.0", "::", "":
		return WildcardAddr
	}
	return host
}

// anyExposed, adreslerden en az biri loopback değilse true döner:
// joker adres veya LAN adresi başka makinelerden erişilebilir demektir.
func anyExposed(addrs []string) bool {
	for _, a := range addrs {
		if a == WildcardAddr {
			return true
		}
		ip := net.ParseIP(a)
		if ip == nil || !ip.IsLoopback() {
			return true
		}
	}
	return false
}
