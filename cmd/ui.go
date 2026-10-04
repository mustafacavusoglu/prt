package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mustafacavusoglu/prt/internal/port"
)

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// painter ANSI renklerini yalnızca terminale yazarken ve kapatılmamışsa uygular.
type painter struct{ on bool }

func newPainter(w io.Writer) painter {
	f, ok := w.(*os.File)
	if !ok || noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return painter{}
	}
	return painter{on: isTerminal(f)}
}

func (p painter) wrap(code, s string) string {
	if !p.on {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (p painter) bold(s string) string   { return p.wrap("1", s) }
func (p painter) yellow(s string) string { return p.wrap("33", s) }
func (p painter) red(s string) string    { return p.wrap("31", s) }
func (p painter) green(s string) string  { return p.wrap("32", s) }
func (p painter) dim(s string) string    { return p.wrap("2", s) }

type tableOptions struct {
	wide     bool // UPTIME, CWD ve komut satırı sütunları
	numbered bool // ilk sütun: seçim numarası
}

// renderTable listeyi hizalı tablo olarak yazar. Hizalamayı bozmamak için renkler
// tabwriter'dan sonra satır bazında uygulanır; ağa açık satırlar sarı olur.
func renderTable(w io.Writer, ls []port.Listener, o tableOptions) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	header := []string{"PORT", "PROTO", "PID", "PROCESS", "USER", "ADDRESS", "SERVICE"}
	if o.wide {
		header = append(header, "UPTIME", "CWD", "COMMAND")
	}
	if o.numbered {
		header = append([]string{"#"}, header...)
	}
	fmt.Fprintln(tw, strings.Join(header, "\t"))
	for i, l := range ls {
		pid := "-"
		if l.PID > 0 {
			pid = fmt.Sprint(l.PID)
		}
		svc := "-"
		if l.BrewService != "" {
			svc = "brew:" + l.BrewService
		}
		row := []string{fmt.Sprint(l.Port), l.Proto, pid, l.Command, l.User, strings.Join(l.Addresses, ","), svc}
		if o.wide {
			row = append(row, formatUptime(l.UptimeSec), shortenHome(l.Cwd), l.Args)
		}
		if o.numbered {
			row = append([]string{fmt.Sprint(i + 1)}, row...)
		}
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()

	p := newPainter(w)
	for i, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		switch {
		case i == 0:
			line = p.bold(line)
		case ls[i-1].Exposed:
			line = p.yellow(line)
		}
		fmt.Fprintln(w, line)
	}
}

func formatUptime(sec int64) string {
	if sec <= 0 {
		return "-"
	}
	d := time.Duration(sec) * time.Second
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%ds", sec)
	}
}

func shortenHome(path string) string {
	if path == "" {
		return "-"
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && home != "/" {
		if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
			if rel == "." {
				return "~"
			}
			return "~/" + rel
		}
	}
	return path
}

func countExposed(ls []port.Listener) int {
	n := 0
	for _, l := range ls {
		if l.Exposed {
			n++
		}
	}
	return n
}
