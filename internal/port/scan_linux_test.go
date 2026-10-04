//go:build linux

package port

import (
	"net"
	"os"
	"testing"
)

func TestParseStatStartTime(t *testing.T) {
	// comm alanında boşluk ve parantez var; 22. alan 123456 olmalı.
	stat := "42 (my (weird) proc) S 1 42 42 0 -1 4194560 100 0 0 0 1 2 0 0 20 0 1 0 123456 1000 100 18446744073709551615"
	got, ok := parseStatStartTime(stat)
	if !ok || got != 123456 {
		t.Errorf("got %d, %v; want 123456", got, ok)
	}
	if _, ok := parseStatStartTime("garbage"); ok {
		t.Error("garbage must fail")
	}
}

// Gerçek bir soket açıp Scan'in onu bu süreçle eşleştirdiğini doğrular.
func TestScanFindsOwnListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("soket açılamadı:", err)
	}
	defer ln.Close()
	want := ln.Addr().(*net.TCPAddr).Port

	ls, err := Scan(ScanOptions{Details: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range ls {
		if l.Port == want && l.PID == os.Getpid() {
			if l.Proto != "tcp" || l.Exposed || len(l.Addresses) != 1 || l.Addresses[0] != "127.0.0.1" {
				t.Errorf("beklenmeyen kayıt: %+v", l)
			}
			if l.Command == "" || l.Command == "-" || l.Args == "" || l.Cwd == "" {
				t.Errorf("süreç bilgisi eksik: %+v", l)
			}
			if !StillListening(l) {
				t.Error("StillListening false döndü")
			}
			return
		}
	}
	t.Fatalf("port %d Scan sonucunda yok (PID %d)", want, os.Getpid())
}

func TestIsAliveSelfAndGone(t *testing.T) {
	if !IsAlive(os.Getpid()) {
		t.Error("kendi sürecimiz canlı olmalı")
	}
	if IsAlive(1 << 22) { // pid_max'in üstünde
		t.Error("olmayan PID canlı görünüyor")
	}
	if err := Terminate(1, SigTERM); err == nil {
		t.Error("PID 1 kapatılamamalı")
	}
	if err := Terminate(1<<22, SigTERM); err != ErrNoProcess {
		t.Errorf("err = %v; ErrNoProcess bekleniyordu", err)
	}
}
