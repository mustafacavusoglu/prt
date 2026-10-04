package cmd

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/mustafacavusoglu/prt/internal/port"
	"github.com/spf13/cobra"
)

func newInteractiveCmd() *cobra.Command {
	o := newKillOptions()
	var all, udp bool
	c := &cobra.Command{
		Use:     "interactive",
		Short:   "Portları numaralı listele ve kapatılacakları seçtir",
		Aliases: []string{"i", "pick"},
		Args:    cobra.NoArgs,
		Example: `  prt interactive
  prt i --all --udp`,
		RunE: func(cmd *cobra.Command, args []string) error { return runInteractive(cmd, o, all, udp) },
	}
	addKillFlags(c, &o)
	c.Flags().BoolVarP(&all, "all", "a", false, "diğer kullanıcıların süreçlerini de göster")
	c.Flags().BoolVarP(&udp, "udp", "u", false, "UDP soketlerini de listele")
	return c
}

func init() { rootCmd.AddCommand(newInteractiveCmd()) }

func runInteractive(cmd *cobra.Command, o killOptions, all, udp bool) error {
	out := cmd.OutOrStdout()
	ls, err := scanFn(port.ScanOptions{UDP: udp, Details: true})
	if err != nil {
		return err
	}
	ls, _ = restrictToUser(ls, all)
	if len(ls) == 0 {
		fmt.Fprintln(out, "Dinlenen port bulunamadı.")
		return nil
	}
	renderTable(out, ls, tableOptions{numbered: true})

	in := bufio.NewReader(cmd.InOrStdin())
	fmt.Fprint(out, "\nKapatılacak satırlar (örn. 1,3 veya 2-4; q = çık): ")
	line, err := in.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" || strings.EqualFold(line, "q") {
		if err != nil {
			fmt.Fprintln(out)
		}
		return nil
	}
	idx, perr := parseSelection(line, len(ls))
	if perr != nil {
		return fail(exitGeneric, "%v", perr)
	}
	picked := make([]port.Listener, len(idx))
	for i, n := range idx {
		picked[i] = ls[n-1]
	}
	fmt.Fprintln(out)
	return killListeners(cmd, in, picked, o)
}

// parseSelection "1,3,5-7" biçimini 1 tabanlı, tekrarsız ve sıralı indekslere çevirir.
func parseSelection(s string, max int) ([]int, error) {
	seen := map[int]bool{}
	var out []int
	add := func(n int) error {
		if n < 1 || n > max {
			return fmt.Errorf("geçersiz satır numarası: %d (1-%d)", n, max)
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
		return nil
	}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		lo, hi, isRange := strings.Cut(part, "-")
		a, err := strconv.Atoi(lo)
		if err != nil {
			return nil, fmt.Errorf("geçersiz seçim: %q", part)
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(hi); err != nil || b < a {
				return nil, fmt.Errorf("geçersiz aralık: %q", part)
			}
		}
		for n := a; n <= b; n++ {
			if err := add(n); err != nil {
				return nil, err
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("seçim boş")
	}
	return out, nil
}
