//go:build linux

package port

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// userHZ, /proc/<pid>/stat içindeki zaman birimidir (neredeyse her zaman 100).
const userHZ = 100

// scan lsof'a ihtiyaç duymadan /proc'u doğrudan okur.
func scan(opts ScanOptions) ([]Listener, error) {
	var socks []procSock
	files := []struct{ path, proto string }{
		{"net/tcp", "tcp"}, {"net/tcp6", "tcp"},
	}
	if opts.UDP {
		files = append(files, struct{ path, proto string }{"net/udp", "udp"}, struct{ path, proto string }{"net/udp6", "udp"})
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join("/proc", f.path))
		if err != nil {
			// IPv6 kapalıysa tcp6/udp6 yoktur; IPv4 dosyaları ise zorunludur.
			if errors.Is(err, fs.ErrNotExist) && strings.HasSuffix(f.path, "6") {
				continue
			}
			return nil, fmt.Errorf("/proc okunamadı: %w", err)
		}
		socks = append(socks, parseProcNet(string(data), f.proto)...)
	}
	if len(socks) == 0 {
		return nil, nil
	}

	want := make(map[uint64]bool, len(socks))
	for _, s := range socks {
		want[s.inode] = true
	}
	owners := socketOwners(want)

	procs := make(map[int]*procDetails)
	getProc := func(pid int) *procDetails {
		if p, ok := procs[pid]; ok {
			return p
		}
		p := readProc(pid, opts.Details)
		procs[pid] = p
		return p
	}

	var out []Listener
	for _, s := range socks {
		pids := owners[s.inode]
		if len(pids) == 0 {
			// Süreç başka bir kullanıcıya ait ve /proc/<pid>/fd okunamıyor.
			out = append(out, Listener{
				PID: 0, Command: "-", User: userName(s.uid), UID: s.uid,
				Port: s.port, Proto: s.proto, Addresses: []string{s.host},
			})
			continue
		}
		for _, pid := range pids {
			p := getProc(pid)
			out = append(out, Listener{
				PID: pid, Command: p.command, Args: p.args, User: userName(p.uid), UID: p.uid,
				Port: s.port, Proto: s.proto, Addresses: []string{s.host},
				Cwd: p.cwd, UptimeSec: p.uptimeSec,
			})
		}
	}
	return out, nil
}

// socketOwners inode -> soketi açık tutan PID'ler haritasını çıkarır.
func socketOwners(want map[uint64]bool) map[uint64][]int {
	owners := make(map[uint64][]int)
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return owners
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		fdDir := filepath.Join("/proc", e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // yetki yok veya süreç bu arada kapandı
		}
		seen := make(map[uint64]bool)
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil || !strings.HasPrefix(link, "socket:[") {
				continue
			}
			inode, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]"), 10, 64)
			if err != nil || !want[inode] || seen[inode] {
				continue
			}
			seen[inode] = true
			owners[inode] = append(owners[inode], pid)
		}
	}
	return owners
}

type procDetails struct {
	command   string
	args      string
	uid       int
	cwd       string
	uptimeSec int64
}

func readProc(pid int, details bool) *procDetails {
	dir := filepath.Join("/proc", strconv.Itoa(pid))
	p := &procDetails{uid: -1}
	if fi, err := os.Stat(dir); err == nil {
		if st, ok := fi.Sys().(*syscall.Stat_t); ok {
			p.uid = int(st.Uid)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "comm")); err == nil {
		p.command = strings.TrimSpace(string(b))
	}
	// comm çekirdekte 15 karaktere kesilir; kesilmiş olabilirse exe adını dene.
	if len(p.command) >= 15 || p.command == "" {
		if exe, err := os.Readlink(filepath.Join(dir, "exe")); err == nil {
			p.command = filepath.Base(strings.TrimSuffix(exe, " (deleted)"))
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil {
		p.args = strings.TrimSpace(strings.ReplaceAll(strings.TrimRight(string(b), "\x00"), "\x00", " "))
	}
	if p.command == "" {
		p.command = "-"
	}
	p.uptimeSec = uptimeSeconds(dir)
	if details {
		p.cwd, _ = os.Readlink(filepath.Join(dir, "cwd"))
	}
	return p
}

// uptimeSeconds /proc/<pid>/stat'taki başlangıç zamanını sistem çalışma süresiyle karşılaştırır.
func uptimeSeconds(procDir string) int64 {
	stat, err := os.ReadFile(filepath.Join(procDir, "stat"))
	if err != nil {
		return 0
	}
	start, ok := parseStatStartTime(string(stat))
	if !ok {
		return 0
	}
	up, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	sysUp, err := strconv.ParseFloat(strings.Fields(string(up))[0], 64)
	if err != nil {
		return 0
	}
	if d := int64(sysUp) - int64(start/userHZ); d > 0 {
		return d
	}
	return 0
}

// parseStatStartTime /proc/<pid>/stat içindeki 22. alanı (starttime, tick) okur.
// Komut adı (2. alan) boşluk ve parantez içerebildiğinden son ')' sonrasından sayılır.
func parseStatStartTime(stat string) (uint64, bool) {
	i := strings.LastIndexByte(stat, ')')
	if i < 0 {
		return 0, false
	}
	f := strings.Fields(stat[i+1:])
	// f[0] 3. alan (state); starttime 22. alan -> f[19].
	if len(f) < 20 {
		return 0, false
	}
	v, err := strconv.ParseUint(f[19], 10, 64)
	return v, err == nil
}

var userCache = map[int]string{}

func userName(uid int) string {
	if uid < 0 {
		return "-"
	}
	if n, ok := userCache[uid]; ok {
		return n
	}
	name := strconv.Itoa(uid)
	if u, err := user.LookupId(name); err == nil {
		name = u.Username
	}
	userCache[uid] = name
	return name
}

// zombie süreç ölmüş ama üst süreç tarafından toplanmamışsa true döner.
func zombie(pid int) bool {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	s := string(b)
	i := strings.LastIndexByte(s, ')')
	return i >= 0 && i+2 < len(s) && s[i+2] == 'Z'
}
