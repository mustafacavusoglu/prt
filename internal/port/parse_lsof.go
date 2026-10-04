package port

import (
	"strconv"
	"strings"
	"time"
)

// splitHostPort "host:port" biçimindeki lsof/netstat adreslerini ayırır.
// IPv6 için köşeli parantezli ("[::1]:80") ve parantezsiz ("::1:80") biçimleri tanır.
// Port "*" ise (bağlanmamış UDP soketi) ok=false döner.
func splitHostPort(s string) (host string, port int, ok bool) {
	i := strings.LastIndexByte(s, ':')
	if i < 0 {
		return "", 0, false
	}
	p, err := strconv.Atoi(s[i+1:])
	if err != nil || p < 1 || p > 65535 {
		return "", 0, false
	}
	return normalizeHost(s[:i]), p, true
}

// parseLsofFields `lsof -F pcLufPn` çıktısını ayrıştırır.
//
// Çıktı satır başına bir alan içerir: p (PID) bir süreç kümesini, f (fd) bir
// dosya kümesini başlatır; c, L, u süreç; P ve n dosya alanlarıdır.
func parseLsofFields(data string) []Listener {
	var (
		out        []Listener
		pid        int
		command    string
		user       string
		uid        = -1
		inFile     bool
		proto, nam string
	)
	flush := func() {
		defer func() { inFile, proto, nam = false, "", "" }()
		if !inFile || nam == "" || pid == 0 {
			return
		}
		if proto != "tcp" && proto != "udp" {
			return
		}
		// "a:1->b:2" bağlı soket; yalnızca dinleyenleri istiyoruz.
		if strings.Contains(nam, "->") {
			return
		}
		host, port, ok := splitHostPort(nam)
		if !ok {
			return
		}
		out = append(out, Listener{
			PID: pid, Command: command, User: user, UID: uid,
			Port: port, Proto: proto, Addresses: []string{host},
		})
	}
	for _, line := range strings.Split(data, "\n") {
		if line == "" {
			continue
		}
		val := line[1:]
		switch line[0] {
		case 'p':
			flush()
			pid, _ = strconv.Atoi(val)
			command, user, uid = "", "", -1
		case 'c':
			command = val
		case 'L':
			user = val
		case 'u':
			if n, err := strconv.Atoi(val); err == nil {
				uid = n
			}
		case 'f':
			flush()
			inFile = true
		case 'P':
			proto = strings.ToLower(val)
		case 'n':
			nam = val
		}
	}
	flush()
	return out
}

// parseLsofCwd `lsof -a -d cwd -F pn` çıktısından PID -> dizin haritası çıkarır.
func parseLsofCwd(data string) map[int]string {
	m := make(map[int]string)
	pid := 0
	for _, line := range strings.Split(data, "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'n':
			if pid != 0 {
				m[pid] = line[1:]
			}
		}
	}
	return m
}

type psEntry struct {
	args   string
	uptime time.Duration
}

// parsePS `ps -ww -o pid=,etime=,args=` çıktısını ayrıştırır.
func parsePS(data string) map[int]psEntry {
	m := make(map[int]psEntry)
	for _, line := range strings.Split(data, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		// args boşluk içerebilir: ilk iki alandan sonrasını ham metinden al.
		rest := strings.TrimSpace(line)
		rest = strings.TrimSpace(strings.TrimPrefix(rest, f[0]))
		rest = strings.TrimSpace(strings.TrimPrefix(rest, f[1]))
		m[pid] = psEntry{args: rest, uptime: parseEtime(f[1])}
	}
	return m
}

// parseEtime ps'in "[[dd-]hh:]mm:ss" biçimindeki süresini ayrıştırır.
func parseEtime(s string) time.Duration {
	var days int
	if d, rest, ok := strings.Cut(s, "-"); ok {
		days, _ = strconv.Atoi(d)
		s = rest
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	nums := make([]int, 0, 3)
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0
		}
		nums = append(nums, n)
	}
	var h, m, sec int
	if len(nums) == 3 {
		h, m, sec = nums[0], nums[1], nums[2]
	} else {
		m, sec = nums[0], nums[1]
	}
	return time.Duration(days)*24*time.Hour + time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
}
