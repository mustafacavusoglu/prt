package port

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Yayınlanan konteyner portlarını Docker/OrbStack'in VM veya proxy süreci dinler.
// Bu sürece sinyal göndermek tek konteyneri değil, tüm Docker'ı kapatır.
var dockerForwarders = map[string]bool{
	"orbstack helper": true, "orbstack": true, // OrbStack
	"com.docker.backend": true, "vpnkit": true, "com.docker.vpnkit": true, "vpnkit-bridge": true, // Docker Desktop
	"docker-proxy": true, // Linux dockerd
}

// IsDockerForwarder, l'yi dinleyen süreç bir Docker/OrbStack port yönlendiricisiyse true döner.
func IsDockerForwarder(l Listener) bool {
	return dockerForwarders[strings.TrimSuffix(strings.ToLower(l.Command), ".exe")]
}

// annotateContainers yönlendirici süreçlerin tuttuğu portlara, o portu yayınlayan konteynerin adını yazar.
// `docker ps` yalnızca böyle bir dinleyici varsa çağrılır.
func annotateContainers(ls []Listener) {
	var byPort map[string]string
	for i := range ls {
		if !IsDockerForwarder(ls[i]) {
			continue
		}
		if byPort == nil {
			byPort = dockerContainers()
		}
		ls[i].Container = byPort[fmt.Sprintf("%d/%s", ls[i].Port, ls[i].Proto)]
	}
}

func dockerContainers() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.Names}}\t{{.Ports}}").Output()
	if err != nil {
		return map[string]string{}
	}
	return parseDockerPorts(string(out))
}

// parseDockerPorts `docker ps --format '{{.Names}}\t{{.Ports}}'` çıktısından
// "port/proto" -> konteyner adı haritası çıkarır. Ports örneği:
// "0.0.0.0:8080->80/tcp, [::]:8080->80/tcp, 0.0.0.0:9000-9001->9000-9001/udp, 6379/tcp".
func parseDockerPorts(out string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		name, ports, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		for _, p := range strings.Split(ports, ",") {
			host, ctr, ok := strings.Cut(strings.TrimSpace(p), "->")
			if !ok { // yayınlanmamış port
				continue
			}
			_, proto, _ := strings.Cut(ctr, "/")
			lo, hi, ok := parsePortRange(host[strings.LastIndexByte(host, ':')+1:])
			if !ok || proto == "" {
				continue
			}
			for n := lo; n <= hi; n++ {
				m[fmt.Sprintf("%d/%s", n, proto)] = name
			}
		}
	}
	return m
}

// parsePortRange "8080" veya "9000-9001" biçimini ayrıştırır.
func parsePortRange(s string) (lo, hi int, ok bool) {
	a, b, isRange := strings.Cut(s, "-")
	lo, err1 := strconv.Atoi(a)
	hi, err2 := lo, error(nil)
	if isRange {
		hi, err2 = strconv.Atoi(b)
	}
	return lo, hi, err1 == nil && err2 == nil && lo >= 1 && lo <= hi && hi <= 65535
}

// StopContainer `docker stop <name>` komutunu çalıştırır.
func StopContainer(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "stop", name).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
