package port

import (
	"fmt"
	"strconv"
	"strings"
)

// PortRange kapalı aralıktır: Lo <= port <= Hi.
type PortRange struct{ Lo, Hi int }

// Contains p'nin aralıkta olup olmadığını söyler.
func (r PortRange) Contains(p int) bool { return p >= r.Lo && p <= r.Hi }

func (r PortRange) String() string {
	if r.Lo == r.Hi {
		return strconv.Itoa(r.Lo)
	}
	return fmt.Sprintf("%d-%d", r.Lo, r.Hi)
}

// ParsePorts "5432", "3000-3010" ve virgülle ayrılmış birleşimlerini ("80,443,8000-8010") ayrıştırır.
func ParsePorts(args []string) ([]PortRange, error) {
	var out []PortRange
	for _, arg := range args {
		for _, part := range strings.Split(arg, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			r, err := parseRange(part)
			if err != nil {
				return nil, err
			}
			out = append(out, r)
		}
	}
	return out, nil
}

func parseRange(s string) (PortRange, error) {
	lo, hi, isRange := strings.Cut(s, "-")
	a, err := parsePortNum(lo)
	if err != nil {
		return PortRange{}, fmt.Errorf("geçersiz port: %q", s)
	}
	if !isRange {
		return PortRange{a, a}, nil
	}
	b, err := parsePortNum(hi)
	if err != nil || b < a {
		return PortRange{}, fmt.Errorf("geçersiz port aralığı: %q", s)
	}
	return PortRange{a, b}, nil
}

func parsePortNum(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > 65535 {
		return 0, fmt.Errorf("geçersiz port")
	}
	return n, nil
}

// Filter Listener listelerini daraltır; sıfır değerli Filter her şeyi geçirir.
type Filter struct {
	Ports       []PortRange
	Name        string // büyük/küçük harf duyarsız; süreç adı, argümanlar ve brew servisinde aranır
	ExposedOnly bool   // yalnızca loopback dışına açık olanlar
	OnlyUID     bool   // true ise yalnızca UID'ye ait süreçler
	UID         int
}

// Match l'nin filtreden geçip geçmediğini söyler.
func (f Filter) Match(l Listener) bool {
	if len(f.Ports) > 0 {
		ok := false
		for _, r := range f.Ports {
			if r.Contains(l.Port) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if f.Name != "" {
		n := strings.ToLower(f.Name)
		hay := strings.ToLower(l.Command + "\x00" + l.Args + "\x00" + l.BrewService)
		if !strings.Contains(hay, n) {
			return false
		}
	}
	if f.ExposedOnly && !l.Exposed {
		return false
	}
	// Windows'ta UID bilinmez (-1); orada kullanıcı filtresi uygulanmaz.
	if f.OnlyUID && f.UID >= 0 && l.UID >= 0 && l.UID != f.UID {
		return false
	}
	return true
}

// Apply geçen kayıtları yeni bir dilim olarak döndürür.
func (f Filter) Apply(in []Listener) []Listener {
	out := make([]Listener, 0, len(in))
	for _, l := range in {
		if f.Match(l) {
			out = append(out, l)
		}
	}
	return out
}
