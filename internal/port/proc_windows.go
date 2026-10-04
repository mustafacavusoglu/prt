//go:build windows

package port

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Terminate taskkill ile süreci kapatır. SigKILL zorla (/F) sonlandırır;
// diğer sinyaller kapatma isteği gönderir (konsol uygulamaları için çoğu zaman yetmez).
func Terminate(pid int, sig Signal) error {
	if pid <= 4 {
		return fmt.Errorf("PID %d kapatılamaz", pid)
	}
	args := []string{"/PID", strconv.Itoa(pid)}
	if sig == SigKILL {
		args = append(args, "/F")
	}
	out, err := exec.Command("taskkill", args...).CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "access is denied") || strings.Contains(low, "erişim engellendi"):
		return fmt.Errorf("PID %d: %w", pid, os.ErrPermission)
	case strings.Contains(low, "not found") || strings.Contains(low, "bulunamadı"):
		return ErrNoProcess
	}
	return fmt.Errorf("taskkill: %s", msg)
}

// IsAlive süreç hâlâ çalışıyorsa true döner.
func IsAlive(pid int) bool {
	out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}
	_, ok := parseTasklist(string(out))[pid]
	return ok
}
