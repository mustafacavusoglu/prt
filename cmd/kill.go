package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mustafacavusoglu/prt/internal/port"
	"github.com/spf13/cobra"
)

// Platform bağımlı işlemler; testlerde sahteleriyle değiştirilir.
var (
	terminateFn = port.Terminate
	aliveFn     = port.IsAlive
	brewStopFn  = port.StopBrewService
	stillFn     = port.StillListening
)

// Sistem servisini yanlışlıkla kapatmayı önlemek için varsayılan olarak reddedilen süreçler.
var protectedNames = map[string]bool{
	"launchd": true, "systemd": true, "init": true, "sshd": true,
	"dockerd": true, "containerd": true, "kernel_task": true,
	"windowserver": true, "loginwindow": true, "system": true, "svchost": true,
}

type killOptions struct {
	force, yes, dryRun, noBrew, unsafe bool
	signal                             string
	timeout                            time.Duration
}

func newKillOptions() killOptions { return killOptions{timeout: 3 * time.Second} }

func addKillFlags(c *cobra.Command, o *killOptions) {
	f := c.Flags()
	f.BoolVarP(&o.force, "force", "f", false, "SIGKILL ile zorla kapat (--signal KILL ile aynı)")
	f.StringVarP(&o.signal, "signal", "s", "", "gönderilecek sinyal: TERM (varsayılan), KILL, HUP, INT")
	f.BoolVarP(&o.yes, "yes", "y", false, "onay sormadan kapat")
	f.BoolVarP(&o.dryRun, "dry-run", "d", false, "hiçbir şey kapatmadan ne olacağını göster")
	f.DurationVar(&o.timeout, "timeout", o.timeout, "sinyalden sonra sürecin kapanmasını bekleme süresi")
	f.BoolVar(&o.noBrew, "no-brew", false, "brew servisi olsa bile `brew services stop` yerine doğrudan sinyal gönder")
	f.BoolVar(&o.unsafe, "unsafe", false, "korumalı sistem süreçlerinin (sshd, systemd, launchd...) kapatılmasına izin ver")
	_ = c.RegisterFlagCompletionFunc("signal", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"TERM", "KILL", "HUP", "INT"}, cobra.ShellCompDirectiveNoFileComp
	})
}

func newKillCmd() *cobra.Command {
	o := newKillOptions()
	c := &cobra.Command{
		Use:     "kill <port|aralık> [port|aralık ...]",
		Short:   "Belirtilen portları dinleyen süreçleri kapat",
		Aliases: []string{"k"},
		Args:    cobra.MinimumNArgs(1),
		Long: `Belirtilen portlarda dinleyen süreçleri kapatır.

Varsayılan olarak SIGTERM gönderir, önce ne kapatılacağını gösterir ve onay ister.
Süreç --timeout içinde kapanmazsa sizi uyarır; zorla kapatmak için -f kullanın.
brew servisi olarak çalışan süreçler 'brew services stop' ile durdurulur.`,
		Example: `  prt kill 6379
  prt kill 3000 5432 8080
  prt kill 3000-3010 --yes
  prt kill 8080 -f            # SIGKILL
  prt kill 8080 --dry-run`,
		ValidArgsFunction: completePorts,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKill(cmd, bufio.NewReader(cmd.InOrStdin()), args, o)
		},
	}
	addKillFlags(c, &o)
	return c
}

func init() { rootCmd.AddCommand(newKillCmd()) }

func completePorts(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	ls, err := scanFn(port.ScanOptions{UDP: true})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	seen := map[int]bool{}
	for _, l := range ls {
		if !seen[l.Port] {
			seen[l.Port] = true
			out = append(out, fmt.Sprintf("%d\t%s", l.Port, l.Command))
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func runKill(cmd *cobra.Command, in *bufio.Reader, args []string, o killOptions) error {
	ranges, err := port.ParsePorts(args)
	if err != nil {
		return fail(exitGeneric, "%v", err)
	}
	all, err := scanFn(port.ScanOptions{UDP: true, Details: true})
	if err != nil {
		return err
	}
	var targets []port.Listener
	for _, r := range ranges {
		n := len(targets)
		for _, l := range all {
			if r.Contains(l.Port) {
				targets = append(targets, l)
			}
		}
		if len(targets) == n {
			fmt.Fprintf(cmd.ErrOrStderr(), "Uyarı: %s üzerinde dinleyen bir süreç bulunamadı.\n", r)
		}
	}
	if len(targets) == 0 {
		return fail(exitNotFound, "belirtilen portlarda dinleyen süreç bulunamadı")
	}
	return killListeners(cmd, in, targets, o)
}

// group aynı PID'ye ait dinleyicileri toplar; bir süreç birden çok portu tutabilir.
type group struct {
	pid       int
	listeners []port.Listener
}

func (g group) first() port.Listener { return g.listeners[0] }

func (g group) portList() string {
	parts := make([]string, len(g.listeners))
	for i, l := range g.listeners {
		parts[i] = fmt.Sprintf("%d/%s", l.Port, l.Proto)
	}
	return strings.Join(parts, ", ")
}

func groupByPID(ls []port.Listener) []group {
	var groups []group
	idx := map[int]int{}
	for _, l := range ls {
		i, ok := idx[l.PID]
		if !ok {
			i = len(groups)
			idx[l.PID] = i
			groups = append(groups, group{pid: l.PID})
		}
		groups[i].listeners = append(groups[i].listeners, l)
	}
	return groups
}

// neverKill PID 1 ve prt'nin kendisi için true döner; --unsafe bile bunları açmaz.
func neverKill(l port.Listener) bool { return l.PID <= 1 || l.PID == os.Getpid() }

func isProtected(l port.Listener) bool {
	return protectedNames[strings.TrimSuffix(strings.ToLower(l.Command), ".exe")]
}

func resolveSignal(o killOptions) (port.Signal, error) {
	if o.signal != "" && o.force {
		if s, err := port.ParseSignal(o.signal); err != nil || s != port.SigKILL {
			return 0, fail(exitGeneric, "-f ile --signal %s birlikte kullanılamaz", o.signal)
		}
	}
	if o.force {
		return port.SigKILL, nil
	}
	if o.signal == "" {
		return port.SigTERM, nil
	}
	s, err := port.ParseSignal(o.signal)
	if err != nil {
		return 0, fail(exitGeneric, "%v", err)
	}
	return s, nil
}

func killListeners(cmd *cobra.Command, in *bufio.Reader, targets []port.Listener, o killOptions) error {
	out, errw := cmd.OutOrStdout(), cmd.ErrOrStderr()
	p := newPainter(out)
	sig, err := resolveSignal(o)
	if err != nil {
		return err
	}

	// 1) Süreçlere göre grupla; yetki/koruma nedeniyle kapatılamayacakları ayır.
	var groups []group
	var blocked []string
	needsSudo := false
	for _, g := range groupByPID(targets) {
		l := g.first()
		switch {
		case g.pid == 0:
			needsSudo = true
			blocked = append(blocked, fmt.Sprintf("%s portunu tutan süreç görülemiyor (başka kullanıcıya ait olabilir)", g.portList()))
		case neverKill(l):
			blocked = append(blocked, fmt.Sprintf("PID %d (%s) hiçbir koşulda kapatılamaz", g.pid, l.Command))
		case isProtected(l) && !o.unsafe:
			blocked = append(blocked, fmt.Sprintf("PID %d (%s) korumalı bir süreç; yine de kapatmak için --unsafe kullanın", g.pid, l.Command))
		default:
			groups = append(groups, g)
		}
	}
	for _, b := range blocked {
		fmt.Fprintln(errw, p.yellow("Atlandı: ")+b)
	}
	if len(groups) == 0 {
		if needsSudo {
			return fail(exitPermission, "kapatılacak süreç yok; sudo ile tekrar deneyin")
		}
		return fail(exitGeneric, "kapatılacak süreç yok")
	}

	// 2) Ne olacağını göster.
	brewMode := func(l port.Listener) bool { return l.BrewService != "" && !o.noBrew }
	for _, g := range groups {
		l := g.first()
		fmt.Fprintf(out, "• %s (PID %d, kullanıcı: %s) — %s\n", p.bold(l.Command), g.pid, l.User, g.portList())
		if l.Args != "" {
			fmt.Fprintf(out, "  %s\n", p.dim(truncate(l.Args, 100)))
		}
		if l.Exposed {
			fmt.Fprintln(out, "  "+p.yellow("⚠ ağdan erişilebilir ("+strings.Join(l.Addresses, ", ")+")"))
		}
		if brewMode(l) {
			fmt.Fprintf(out, "  brew servisi olarak yönetiliyor → brew services stop %s\n", l.BrewService)
			if o.force || o.signal != "" {
				fmt.Fprintln(out, "  (brew servislerinde sinyal seçimi yok sayılır; doğrudan sinyal için --no-brew)")
			}
		}
	}

	if o.dryRun {
		fmt.Fprintf(out, "\nKuru çalıştırma: %d süreç kapatılacaktı (%s). Hiçbir şey yapılmadı.\n", len(groups), sig)
		return nil
	}

	// 3) Onay.
	if !o.yes {
		fmt.Fprintf(out, "\n%d süreç kapatılsın mı? [y/N]: ", len(groups))
		answer, err := in.ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" && answer != "e" && answer != "evet" {
			if err != nil && answer == "" {
				fmt.Fprintln(out)
				fmt.Fprintln(errw, "Onay alınamadı (girdi yok); etkileşimsiz kullanımda --yes ekleyin.")
			}
			fmt.Fprintln(out, "İptal edildi.")
			return &exitError{code: exitCancelled}
		}
	}

	// 4) Onay süresince durum değişmiş olabilir (PID yeniden kullanımı); göndermeden önce doğrula.
	var perm, failed bool
	var pending []group
	for _, g := range groups {
		l := g.first()
		if !stillFn(l) {
			fmt.Fprintf(out, "%s %s (PID %d) artık dinlemiyor, atlandı.\n", p.dim("-"), l.Command, g.pid)
			continue
		}
		var err error
		if brewMode(l) {
			err = brewStopFn(l.BrewService)
		} else {
			err = terminateFn(g.pid, sig)
		}
		switch {
		case err == nil:
			pending = append(pending, g)
		case errors.Is(err, port.ErrNoProcess):
			fmt.Fprintf(out, "%s %s (PID %d) zaten kapanmış.\n", p.green("✓"), l.Command, g.pid)
		case errors.Is(err, os.ErrPermission):
			perm = true
			fmt.Fprintf(errw, "%s %s (PID %d): yetki yok — sudo ile tekrar deneyin\n", p.red("✗"), l.Command, g.pid)
		default:
			failed = true
			fmt.Fprintf(errw, "%s %s (PID %d): %v\n", p.red("✗"), l.Command, g.pid, err)
		}
	}

	// 5) Sürecin gerçekten kapandığını doğrula.
	survivors := waitForExit(pending, o.timeout)
	dead := map[int]bool{}
	for _, g := range pending {
		dead[g.pid] = true
	}
	for _, g := range survivors {
		delete(dead, g.pid)
	}
	for _, g := range pending {
		if !dead[g.pid] {
			continue
		}
		l := g.first()
		if brewMode(l) {
			fmt.Fprintf(out, "%s %s durduruldu (brew services stop %s) — %s\n", p.green("✓"), l.Command, l.BrewService, g.portList())
		} else {
			fmt.Fprintf(out, "%s %s (PID %d) kapatıldı [%s] — %s\n", p.green("✓"), l.Command, g.pid, sig, g.portList())
		}
	}
	for _, g := range survivors {
		l := g.first()
		fmt.Fprintf(errw, "%s %s (PID %d) hâlâ çalışıyor; zorla kapatmak için: prt kill %d -f\n", p.yellow("!"), l.Command, g.pid, l.Port)
	}

	switch {
	case perm:
		return &exitError{code: exitPermission}
	case failed:
		return &exitError{code: exitGeneric}
	case len(survivors) > 0:
		return &exitError{code: exitSurvived}
	}
	return nil
}

// waitForExit timeout dolana kadar süreçlerin kapanmasını bekler ve hayatta kalanları döndürür.
func waitForExit(groups []group, timeout time.Duration) []group {
	deadline := time.Now().Add(timeout)
	alive := groups
	for {
		var still []group
		for _, g := range alive {
			if aliveFn(g.pid) {
				still = append(still, g)
			}
		}
		alive = still
		if len(alive) == 0 || !time.Now().Before(deadline) {
			return alive
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// truncate boşlukları (satır sonları dahil) tek aralığa indirip n karakterle sınırlar.
func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
