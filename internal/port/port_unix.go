//go:build darwin || linux

package port

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func resolveCommand(pid int) string {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return ""
	}
	fullPath := strings.TrimSpace(string(out))
	parts := strings.Split(fullPath, "/")
	return parts[len(parts)-1]
}

// macOS'ta launchctl üzerinden homebrew service tespiti yapar.
// Linux'ta launchctl olmadığı için boş döner.
func findBrewService(pid int) string {
	out, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return ""
	}
	pidStr := strconv.Itoa(pid)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		if fields[0] == pidStr && strings.HasPrefix(fields[2], "homebrew.mxcl.") {
			return strings.TrimPrefix(fields[2], "homebrew.mxcl.")
		}
	}
	return ""
}

func GetListeningPorts() ([]ListeningPort, error) {
	out, err := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-n", "-P").CombinedOutput()
	if err != nil {
		if len(out) == 0 {
			return nil, fmt.Errorf("lsof çalıştırılamadı: %w", err)
		}
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return nil, nil
	}

	seen := make(map[string]bool)
	var ports []ListeningPort

	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		command := fields[0]
		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		user := fields[2]

		if fullName := resolveCommand(pid); fullName != "" {
			command = fullName
		}

		nameField := fields[8]
		idx := strings.LastIndex(nameField, ":")
		if idx == -1 {
			continue
		}
		portStr := nameField[idx+1:]
		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		key := fmt.Sprintf("%d:%d", pid, port)
		if seen[key] {
			continue
		}
		seen[key] = true

		ports = append(ports, ListeningPort{
			PID:         pid,
			Command:     command,
			User:        user,
			Port:        port,
			BrewService: findBrewService(pid),
		})
	}

	return ports, nil
}

func FindByPort(portNum int) (*ListeningPort, error) {
	ports, err := GetListeningPorts()
	if err != nil {
		return nil, err
	}
	for _, p := range ports {
		if p.Port == portNum {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("port %d üzerinde dinleyen bir process bulunamadı", portNum)
}

func StopBrewService(serviceName string) error {
	return exec.Command("brew", "services", "stop", serviceName).Run()
}

func KillByPort(portNum int, force bool) (*ListeningPort, error) {
	p, err := FindByPort(portNum)
	if err != nil {
		return nil, err
	}

	if p.BrewService != "" {
		if err := StopBrewService(p.BrewService); err != nil {
			return nil, fmt.Errorf("brew services stop %s başarısız: %w", p.BrewService, err)
		}
		return p, nil
	}

	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}

	if err := syscall.Kill(p.PID, sig); err != nil {
		return nil, fmt.Errorf("PID %d kill edilemedi: %w", p.PID, err)
	}

	return p, nil
}
