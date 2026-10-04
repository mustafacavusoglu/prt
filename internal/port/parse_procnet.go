package port

import (
	"encoding/hex"
	"net"
	"strconv"
	"strings"
)

// procSock /proc/net/{tcp,udp}{,6} içindeki dinleyen bir soketi temsil eder.
type procSock struct {
	host  string
	port  int
	proto string
	uid   int
	inode uint64
}

// parseProcNet /proc/net/tcp, tcp6, udp, udp6 içeriğini ayrıştırır.
//
// TCP için yalnızca LISTEN (0A), UDP için yalnızca bağlanmamış (state 07 ve
// uzak port 0) soketler alınır.
func parseProcNet(data, proto string) []procSock {
	var out []procSock
	lines := strings.Split(data, "\n")
	for _, line := range lines[min(1, len(lines)):] { // ilk satır başlık
		f := strings.Fields(line)
		if len(f) < 10 {
			continue
		}
		state := f[3]
		switch proto {
		case "tcp":
			if state != "0A" {
				continue
			}
		case "udp":
			if state != "07" || !strings.HasSuffix(f[2], ":0000") {
				continue
			}
		}
		host, port, ok := decodeProcAddr(f[1])
		if !ok || port == 0 {
			continue
		}
		uid, _ := strconv.Atoi(f[7])
		inode, err := strconv.ParseUint(f[9], 10, 64)
		if err != nil || inode == 0 {
			continue
		}
		out = append(out, procSock{host: host, port: port, proto: proto, uid: uid, inode: inode})
	}
	return out
}

// decodeProcAddr "0100007F:1770" (IPv4) veya 32 hanelik IPv6 adresini çözer.
// Çekirdek adresi 32 bitlik kelimeler hâlinde, host byte sırasıyla (little-endian) yazar.
func decodeProcAddr(s string) (host string, port int, ok bool) {
	addrHex, portHex, found := strings.Cut(s, ":")
	if !found {
		return "", 0, false
	}
	p, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return "", 0, false
	}
	b, err := hex.DecodeString(addrHex)
	if err != nil || (len(b) != 4 && len(b) != 16) {
		return "", 0, false
	}
	for i := 0; i+4 <= len(b); i += 4 {
		b[i], b[i+1], b[i+2], b[i+3] = b[i+3], b[i+2], b[i+1], b[i]
	}
	return normalizeHost(net.IP(b).String()), int(p), true
}
