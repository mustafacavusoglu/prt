package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mustafacavusoglu/prt/internal/port"
)

// fakeSystem scan/terminate/alive/brew/still işlevlerini değiştirir ve çağrıları kaydeder.
type fakeSystem struct {
	listeners  []port.Listener
	terminated []string // "pid:SIG"
	brewStops  []string
	ignoreTERM map[int]bool // true ise süreç SIGTERM'e rağmen yaşar
	termErr    map[int]error
	dead       map[int]bool
	gone       map[int]bool // artık dinlemiyor
}

func newFake(ls ...port.Listener) *fakeSystem {
	f := &fakeSystem{listeners: ls, ignoreTERM: map[int]bool{}, termErr: map[int]error{}, dead: map[int]bool{}, gone: map[int]bool{}}
	scanFn = func(port.ScanOptions) ([]port.Listener, error) { return f.listeners, nil }
	terminateFn = func(pid int, sig port.Signal) error {
		f.terminated = append(f.terminated, fmt.Sprintf("%d:%s", pid, sig))
		if err := f.termErr[pid]; err != nil {
			return err
		}
		if sig == port.SigKILL || !f.ignoreTERM[pid] {
			f.dead[pid] = true
		}
		return nil
	}
	aliveFn = func(pid int) bool { return !f.dead[pid] }
	brewStopFn = func(name string) error {
		f.brewStops = append(f.brewStops, name)
		for _, l := range f.listeners {
			if l.BrewService == name {
				f.dead[l.PID] = true
			}
		}
		return nil
	}
	stillFn = func(l port.Listener) bool { return !f.gone[l.PID] }
	return f
}

// listener çalıştıran kullanıcıya ait bir kayıt üretir; böylece testler root olsun
// olmasın aynı davranır (list/interactive varsayılan olarak yalnızca kendi süreçlerini gösterir).
func listener(pid int, name string, p int) port.Listener {
	return port.Listener{PID: pid, Command: name, User: "u", UID: os.Getuid(), Port: p, Proto: "tcp", Addresses: []string{"127.0.0.1"}}
}

func execute(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	c := newKillCmd()
	c.SetOut(&out)
	c.SetErr(&errb)
	c.SetIn(strings.NewReader(stdin))
	c.SetArgs(args)
	c.SilenceUsage, c.SilenceErrors = true, true
	err = c.Execute()
	return out.String(), errb.String(), err
}

func codeOf(err error) int {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	if err != nil {
		return exitGeneric
	}
	return 0
}

func TestKillWithYes(t *testing.T) {
	f := newFake(listener(100, "redis-server", 6379), listener(200, "postgres", 5432))
	out, _, err := execute(t, "", "6379", "--yes")
	if err != nil {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if !reflect.DeepEqual(f.terminated, []string{"100:SIGTERM"}) {
		t.Errorf("terminated = %v", f.terminated)
	}
	if !strings.Contains(out, "kapatıldı") {
		t.Errorf("çıktı: %s", out)
	}
}

func TestKillConfirmation(t *testing.T) {
	for answer, wantKill := range map[string]bool{"y\n": true, "YES\n": true, "evet\n": true, "n\n": false, "\n": false, "": false} {
		f := newFake(listener(100, "redis-server", 6379))
		_, _, err := execute(t, answer, "6379", "--timeout", "10ms")
		if got := len(f.terminated) == 1; got != wantKill {
			t.Errorf("cevap %q: kill=%v; want %v", answer, got, wantKill)
		}
		if !wantKill && codeOf(err) != exitCancelled {
			t.Errorf("cevap %q: çıkış kodu %d; want %d", answer, codeOf(err), exitCancelled)
		}
	}
}

func TestKillDryRun(t *testing.T) {
	f := newFake(listener(100, "redis-server", 6379))
	out, _, err := execute(t, "", "6379", "--dry-run")
	if err != nil || len(f.terminated) != 0 {
		t.Fatalf("err=%v terminated=%v", err, f.terminated)
	}
	if !strings.Contains(out, "Hiçbir şey yapılmadı") {
		t.Errorf("çıktı: %s", out)
	}
}

func TestKillNotFoundAndInvalid(t *testing.T) {
	newFake(listener(100, "redis-server", 6379))
	if _, _, err := execute(t, "", "1234", "-y"); codeOf(err) != exitNotFound {
		t.Errorf("kod = %d; want %d", codeOf(err), exitNotFound)
	}
	if _, _, err := execute(t, "", "abc", "-y"); codeOf(err) != exitGeneric {
		t.Errorf("kod = %d; want %d", codeOf(err), exitGeneric)
	}
	if _, _, err := execute(t, "", "6379", "-y", "-s", "USR1"); codeOf(err) != exitGeneric {
		t.Errorf("geçersiz sinyal kabul edildi")
	}
	if _, _, err := execute(t, "", "6379", "-y", "-f", "-s", "HUP"); codeOf(err) != exitGeneric {
		t.Errorf("-f ile -s HUP çakışması yakalanmadı")
	}
}

func TestKillRangeAndDedupByPID(t *testing.T) {
	// 100 iki portu tutuyor: tek kez sinyal almalı.
	f := newFake(listener(100, "node", 3000), listener(100, "node", 3001), listener(200, "vite", 3002), listener(300, "other", 4000))
	if _, _, err := execute(t, "", "3000-3002", "-y"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.terminated, []string{"100:SIGTERM", "200:SIGTERM"}) {
		t.Errorf("terminated = %v", f.terminated)
	}
}

func TestKillMultipleProcessesOnSamePort(t *testing.T) {
	f := newFake(listener(100, "nginx", 80), listener(101, "nginx", 80))
	if _, _, err := execute(t, "", "80", "-y"); err != nil {
		t.Fatal(err)
	}
	if len(f.terminated) != 2 {
		t.Errorf("terminated = %v; iki süreç de kapatılmalı", f.terminated)
	}
}

func TestKillSurvivorReportsAndForce(t *testing.T) {
	f := newFake(listener(100, "stubborn", 7000))
	f.ignoreTERM[100] = true
	_, errOut, err := execute(t, "", "7000", "-y", "--timeout", "50ms")
	if codeOf(err) != exitSurvived {
		t.Errorf("kod = %d; want %d", codeOf(err), exitSurvived)
	}
	if !strings.Contains(errOut, "prt kill 7000 -f") {
		t.Errorf("ipucu yok: %s", errOut)
	}
	if _, _, err := execute(t, "", "7000", "-y", "-f", "--timeout", "50ms"); err != nil {
		t.Errorf("-f sonrası hata: %v", err)
	}
	if got := f.terminated[len(f.terminated)-1]; got != "100:SIGKILL" {
		t.Errorf("son sinyal = %s", got)
	}
}

func TestKillPermissionDenied(t *testing.T) {
	f := newFake(listener(100, "rootproc", 80))
	f.termErr[100] = fmt.Errorf("PID 100: %w", os.ErrPermission)
	_, errOut, err := execute(t, "", "80", "-y")
	if codeOf(err) != exitPermission || !strings.Contains(errOut, "sudo") {
		t.Errorf("kod=%d stderr=%s", codeOf(err), errOut)
	}
}

func TestKillAlreadyGoneAndRace(t *testing.T) {
	f := newFake(listener(100, "a", 1000), listener(200, "b", 2000))
	f.termErr[100] = port.ErrNoProcess
	f.gone[200] = true // onay sırasında port başkasına geçti / süreç kapandı
	out, _, err := execute(t, "", "1000", "2000", "-y")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out, "zaten kapanmış") || !strings.Contains(out, "artık dinlemiyor") {
		t.Errorf("çıktı: %s", out)
	}
	if reflect.DeepEqual(f.terminated, []string{"100:SIGTERM", "200:SIGTERM"}) {
		t.Error("artık dinlemeyen sürece sinyal gönderilmemeli")
	}
}

func TestKillRefusesProtectedAndUnknownPID(t *testing.T) {
	f := newFake(listener(1, "init", 1111), listener(500, "sshd", 22), listener(0, "-", 2222), listener(os.Getpid(), "prt", 3333))
	for _, p := range []string{"1111", "22", "2222", "3333"} {
		if _, _, err := execute(t, "", p, "-y"); err == nil {
			t.Errorf("port %s kapatılabildi", p)
		}
	}
	if len(f.terminated) != 0 {
		t.Errorf("korumalı süreçlere sinyal gitti: %v", f.terminated)
	}
	if _, _, err := execute(t, "", "2222", "-y"); codeOf(err) != exitPermission {
		t.Errorf("PID 0 için çıkış kodu = %d; want %d", codeOf(err), exitPermission)
	}
	// --unsafe sshd'ye izin verir, PID 1'e ve kendimize asla.
	if _, _, err := execute(t, "", "22", "-y", "--unsafe"); err != nil {
		t.Errorf("--unsafe ile sshd: %v", err)
	}
	if _, _, err := execute(t, "", "1111", "-y", "--unsafe"); err == nil {
		t.Error("PID 1 --unsafe ile bile kapatılmamalı")
	}
}

func TestKillBrewService(t *testing.T) {
	redis := listener(100, "redis-server", 6379)
	redis.BrewService = "redis"
	f := newFake(redis)
	out, _, err := execute(t, "", "6379", "-y")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.brewStops, []string{"redis"}) || len(f.terminated) != 0 {
		t.Errorf("brew=%v term=%v", f.brewStops, f.terminated)
	}
	if !strings.Contains(out, "brew services stop redis") {
		t.Errorf("çıktı: %s", out)
	}

	f = newFake(redis)
	if _, _, err := execute(t, "", "6379", "-y", "--no-brew"); err != nil {
		t.Fatal(err)
	}
	if len(f.brewStops) != 0 || len(f.terminated) != 1 {
		t.Errorf("--no-brew: brew=%v term=%v", f.brewStops, f.terminated)
	}
}

func TestParseSelection(t *testing.T) {
	got, err := parseSelection("3, 1-2 2,5", 5)
	if err != nil || !reflect.DeepEqual(got, []int{3, 1, 2, 5}) {
		t.Errorf("got %v, %v", got, err)
	}
	for _, bad := range []string{"", "0", "6", "a", "3-1", "1-99", ","} {
		if _, err := parseSelection(bad, 5); err == nil {
			t.Errorf("parseSelection(%q) hata vermeli", bad)
		}
	}
}

func TestInteractiveFlow(t *testing.T) {
	f := newFake(listener(100, "redis-server", 6379), listener(200, "postgres", 5432))
	var out, errb bytes.Buffer
	c := newInteractiveCmd()
	c.SetOut(&out)
	c.SetErr(&errb)
	c.SetIn(strings.NewReader("2\ny\n")) // satır 2 (port sırasına göre 6379) ve onay
	c.SetArgs(nil)
	scanFn = func(port.ScanOptions) ([]port.Listener, error) { return f.listeners, nil }
	if err := c.Execute(); err != nil {
		t.Fatalf("err = %v\n%s", err, out.String())
	}
	if len(f.terminated) != 1 {
		t.Errorf("terminated = %v", f.terminated)
	}
	if !strings.Contains(out.String(), "#") {
		t.Errorf("numaralı tablo yok:\n%s", out.String())
	}
}

func TestInteractiveQuit(t *testing.T) {
	f := newFake(listener(100, "redis-server", 6379))
	for _, in := range []string{"q\n", "\n", ""} {
		var out bytes.Buffer
		c := newInteractiveCmd()
		c.SetOut(&out)
		c.SetIn(strings.NewReader(in))
		c.SetArgs(nil)
		if err := c.Execute(); err != nil {
			t.Errorf("girdi %q: %v", in, err)
		}
	}
	if len(f.terminated) != 0 {
		t.Errorf("terminated = %v", f.terminated)
	}
}

func TestListFilteringAndJSON(t *testing.T) {
	exposed := listener(200, "nginx", 8080)
	exposed.Addresses, exposed.Exposed = []string{"*"}, true
	newFake(listener(100, "redis-server", 6379), exposed)

	run := func(args ...string) string {
		var out, errb bytes.Buffer
		c := newListCmd()
		c.SetOut(&out)
		c.SetErr(&errb)
		c.SetArgs(args)
		if err := c.Execute(); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if out := run("--exposed"); !strings.Contains(out, "nginx") || strings.Contains(out, "redis") {
		t.Errorf("--exposed:\n%s", out)
	}
	if out := run("6379"); !strings.Contains(out, "redis") || strings.Contains(out, "nginx") {
		t.Errorf("port filtresi:\n%s", out)
	}
	if out := run("--json", "-n", "zzz"); strings.TrimSpace(out) != "[]" {
		t.Errorf("boş JSON = %q", out)
	}
	if out := run("--json", "-n", "nginx"); !strings.Contains(out, `"process": "nginx"`) || !strings.Contains(out, `"exposed": true`) {
		t.Errorf("JSON:\n%s", out)
	}
}

func TestRestrictToUser(t *testing.T) {
	mine := listener(1, "mine", 1000)
	other := listener(2, "other", 2000)
	other.UID = os.Getuid() + 1
	ls := []port.Listener{mine, other}

	if got, hidden := restrictToUser(ls, true); len(got) != 2 || hidden != 0 {
		t.Errorf("--all: got %d, hidden %d", len(got), hidden)
	}
	got, hidden := restrictToUser(ls, false)
	if os.Getuid() <= 0 { // root ve Windows her şeyi görür
		if len(got) != 2 || hidden != 0 {
			t.Errorf("root: got %d, hidden %d", len(got), hidden)
		}
		return
	}
	if len(got) != 1 || got[0].PID != 1 || hidden != 1 {
		t.Errorf("got %+v, hidden %d", got, hidden)
	}
}

func TestWaitForExit(t *testing.T) {
	calls := 0
	aliveFn = func(int) bool { calls++; return calls < 3 }
	start := time.Now()
	left := waitForExit([]group{{pid: 1}}, 5*time.Second)
	if len(left) != 0 || time.Since(start) > 2*time.Second {
		t.Errorf("left=%v süre=%v", left, time.Since(start))
	}
}

func TestHelpersAndTruncate(t *testing.T) {
	if got := truncate("a  b\nc", 10); got != "a b c" {
		t.Errorf("truncate = %q", got)
	}
	if got := truncate(strings.Repeat("x", 20), 5); got != "xxxx…" {
		t.Errorf("truncate = %q", got)
	}
	for sec, want := range map[int64]string{0: "-", 45: "45s", 125: "2m", 3700: "1h1m", 90000: "1d1h"} {
		if got := formatUptime(sec); got != want {
			t.Errorf("formatUptime(%d) = %q; want %q", sec, got, want)
		}
	}
}
