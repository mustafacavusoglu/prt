//go:build !linux && !windows

package port

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const cmdTimeout = 10 * time.Second

// run bir komutu zaman aşımıyla çalıştırır ve stdout'unu döndürür.
// lsof, eşleşme bulamazsa veya bazı süreçlere erişemezse 1 ile çıkar; stdout doluysa bu hata sayılmaz.
func run(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return "", fmt.Errorf("%s çalıştırılamadı: %w", name, err)
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("%s %s içinde yanıt vermedi", name, cmdTimeout)
		}
		if stdout.Len() == 0 && ee.ExitCode() != 1 {
			return "", fmt.Errorf("%s başarısız: %s", name, strings.TrimSpace(stderr.String()))
		}
	}
	return stdout.String(), nil
}

func scan(opts ScanOptions) ([]Listener, error) {
	if _, err := exec.LookPath("lsof"); err != nil {
		return nil, errors.New("lsof bulunamadı; lütfen kurun ve PATH'e ekleyin")
	}
	base := []string{"-nP", "+c", "0", "-F", "pcLufPn"}
	out, err := run("lsof", append(append([]string{}, base...), "-iTCP", "-sTCP:LISTEN")...)
	if err != nil {
		return nil, err
	}
	ls := parseLsofFields(out)
	if opts.UDP {
		out, err := run("lsof", append(append([]string{}, base...), "-iUDP")...)
		if err != nil {
			return nil, err
		}
		ls = append(ls, parseLsofFields(out)...)
	}
	if len(ls) == 0 {
		return nil, nil
	}

	pids := uniquePIDs(ls)
	info := psInfo(pids)
	brew := brewServices()
	var cwds map[int]string
	if opts.Details {
		cwds = lsofCwds(pids)
	}
	for i := range ls {
		l := &ls[i]
		if e, ok := info[l.PID]; ok {
			l.Args = e.args
			l.UptimeSec = int64(e.uptime.Seconds())
		}
		l.BrewService = brew[l.PID]
		l.Cwd = cwds[l.PID]
	}
	return ls, nil
}

func uniquePIDs(ls []Listener) []int {
	seen := make(map[int]bool, len(ls))
	var pids []int
	for _, l := range ls {
		if !seen[l.PID] {
			seen[l.PID] = true
			pids = append(pids, l.PID)
		}
	}
	return pids
}

func joinPIDs(pids []int) string {
	s := make([]string, len(pids))
	for i, p := range pids {
		s[i] = strconv.Itoa(p)
	}
	return strings.Join(s, ",")
}

// psInfo tüm süreçler için argüman ve çalışma süresini tek `ps` çağrısıyla okur.
func psInfo(pids []int) map[int]psEntry {
	out, err := run("ps", "-ww", "-o", "pid=,etime=,args=", "-p", joinPIDs(pids))
	if err != nil {
		return nil
	}
	return parsePS(out)
}

// lsofCwds çalışma dizinlerini tek `lsof` çağrısıyla okur.
func lsofCwds(pids []int) map[int]string {
	out, err := run("lsof", "-nP", "-a", "-d", "cwd", "-F", "pn", "-p", joinPIDs(pids))
	if err != nil {
		return nil
	}
	return parseLsofCwd(out)
}

// zombie, süreç ölmüş ama üst süreç tarafından toplanmamışsa true döner.
func zombie(pid int) bool {
	out, err := run("ps", "-o", "stat=", "-p", strconv.Itoa(pid))
	return err == nil && strings.HasPrefix(strings.TrimSpace(out), "Z")
}
