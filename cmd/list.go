package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mustafacavusoglu/prt/internal/port"
	"github.com/spf13/cobra"
)

type listOptions struct {
	all, udp, exposed, wide, json bool
	name                          string
}

func newListCmd() *cobra.Command {
	var o listOptions
	c := &cobra.Command{
		Use:     "list [port|aralık ...]",
		Short:   "Açık portları listele",
		Aliases: []string{"ls", "l"},
		Long: `Dinlenen TCP portlarını listeler. Varsayılan olarak yalnızca sizin süreçlerinizi gösterir.

Port veya aralık verirseniz sonuç bunlarla sınırlanır: prt list 5432 3000-3010`,
		Example: `  prt list
  prt list 6379
  prt list --name redis
  prt list --exposed          # yalnızca ağa açık (0.0.0.0 / LAN) olanlar
  prt list --all --udp --wide
  prt list --json | jq '.[] | select(.exposed)'`,
		RunE: func(cmd *cobra.Command, args []string) error { return runList(cmd, args, o) },
	}
	f := c.Flags()
	f.BoolVarP(&o.all, "all", "a", false, "diğer kullanıcıların süreçlerini de göster")
	f.BoolVarP(&o.udp, "udp", "u", false, "UDP soketlerini de listele")
	f.BoolVarP(&o.exposed, "exposed", "e", false, "yalnızca loopback dışına açık (ağdan erişilebilir) portlar")
	f.StringVarP(&o.name, "name", "n", "", "süreç adı/komutuna göre filtrele (büyük/küçük harf duyarsız)")
	f.BoolVarP(&o.wide, "wide", "w", false, "çalışma süresi, dizin ve komut satırını da göster")
	f.BoolVar(&o.json, "json", false, "JSON çıktı üret")
	return c
}

func init() { rootCmd.AddCommand(newListCmd()) }

// scanFn ve diğer *Fn değişkenleri testlerde sahte uygulamalarla değiştirilir.
var scanFn = port.Scan

func runList(cmd *cobra.Command, args []string, o listOptions) error {
	ranges, err := port.ParsePorts(args)
	if err != nil {
		return fail(exitGeneric, "%v", err)
	}
	all, err := scanFn(port.ScanOptions{UDP: o.udp, Details: o.wide || o.json})
	if err != nil {
		return err
	}
	base := port.Filter{Ports: ranges, Name: o.name, ExposedOnly: o.exposed}.Apply(all)
	shown, hidden := restrictToUser(base, o.all)

	out := cmd.OutOrStdout()
	if o.json {
		if shown == nil {
			shown = []port.Listener{}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(shown)
	}

	if len(shown) == 0 {
		fmt.Fprintln(out, "Dinlenen port bulunamadı.")
	} else {
		renderTable(out, shown, tableOptions{wide: o.wide})
		p := newPainter(out)
		summary := fmt.Sprintf("\nToplam: %d port", len(shown))
		if n := countExposed(shown); n > 0 {
			summary += p.yellow(fmt.Sprintf(" (%d tanesi ağa açık)", n))
		}
		fmt.Fprintln(out, summary)
	}
	if hidden > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "%d port başka kullanıcılara ait; hepsini görmek için --all kullanın (gerekirse sudo ile).\n", hidden)
	}
	return nil
}

// restrictToUser root dışındaki kullanıcılar için listeyi kendi süreçleriyle sınırlar.
// Gizlenen kayıt sayısını da döndürür.
func restrictToUser(ls []port.Listener, all bool) (shown []port.Listener, hidden int) {
	uid := os.Getuid()
	if all || uid <= 0 { // root her şeyi görür; Windows'ta uid -1'dir
		return ls, 0
	}
	shown = port.Filter{OnlyUID: true, UID: uid}.Apply(ls)
	return shown, len(ls) - len(shown)
}
