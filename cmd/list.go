package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mustafacavusoglu/prt/internal/port"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Açık TCP portlarını listele",
	Aliases: []string{"ls", "l"},
	RunE: func(cmd *cobra.Command, args []string) error {
		ports, err := port.GetListeningPorts()
		if err != nil {
			return err
		}

		if len(ports) == 0 {
			fmt.Println("Dinlenen port bulunamadı.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "PORT\tPID\tPROCESS\tUSER\tSERVICE")
		fmt.Fprintln(w, "----\t---\t-------\t----\t-------")
		for _, p := range ports {
			svc := "-"
			if p.BrewService != "" {
				svc = "brew:" + p.BrewService
			}
			fmt.Fprintf(w, "%d\t%d\t%s\t%s\t%s\n", p.Port, p.PID, p.Command, p.User, svc)
		}
		w.Flush()

		fmt.Printf("\nToplam: %d port\n", len(ports))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
