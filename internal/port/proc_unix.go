//go:build !windows

package port

import (
	"errors"
	"fmt"
	"syscall"
)

func (s Signal) sys() syscall.Signal {
	switch s {
	case SigKILL:
		return syscall.SIGKILL
	case SigHUP:
		return syscall.SIGHUP
	case SigINT:
		return syscall.SIGINT
	default:
		return syscall.SIGTERM
	}
}

// Terminate pid'ye sinyal gönderir. Süreç zaten yoksa ErrNoProcess döner;
// yetki hatası errors.Is(err, os.ErrPermission) ile ayırt edilebilir.
func Terminate(pid int, sig Signal) error {
	if pid <= 1 {
		return fmt.Errorf("PID %d kapatılamaz", pid)
	}
	err := syscall.Kill(pid, sig.sys())
	if errors.Is(err, syscall.ESRCH) {
		return ErrNoProcess
	}
	if err != nil {
		return fmt.Errorf("PID %d: %w", pid, err)
	}
	return nil
}

// IsAlive süreç hâlâ çalışıyorsa true döner (zombi süreçler ölü sayılır).
func IsAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	return !zombie(pid)
}
