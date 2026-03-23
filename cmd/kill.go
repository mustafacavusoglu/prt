package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mustafacavusoglu/prt/internal/port"
	"github.com/spf13/cobra"
)

var forceKill bool

var killCmd = &cobra.Command{
	Use:   "kill <port>",
	Short: "Belirtilen porttaki process'i kapat",
	Aliases: []string{"k"},
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		portNum, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("geçersiz port numarası: %s", args[0])
		}

		p, err := port.FindByPort(portNum)
		if err != nil {
			return err
		}

		fmt.Printf("Port %d üzerinde çalışan: %s (PID: %d, User: %s)\n", p.Port, p.Command, p.PID, p.User)
		if p.BrewService != "" {
			fmt.Printf("  → brew service olarak yönetiliyor: %s\n", p.BrewService)
		}

		if !forceKill {
			fmt.Print("Bu process kapatılsın mı? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("İptal edildi.")
				return nil
			}
		}

		killed, err := port.KillByPort(portNum, forceKill)
		if err != nil {
			return err
		}

		if killed.BrewService != "" {
			fmt.Printf("✓ brew services stop %s - port %d kapatıldı\n", killed.BrewService, killed.Port)
		} else {
			sig := "SIGTERM"
			if forceKill {
				sig = "SIGKILL"
			}
			fmt.Printf("✓ %s (PID: %d) port %d kapatıldı [%s]\n", killed.Command, killed.PID, killed.Port, sig)
		}
		return nil
	},
}

func init() {
	killCmd.Flags().BoolVarP(&forceKill, "force", "f", false, "SIGKILL ile zorla kapat")
	rootCmd.AddCommand(killCmd)
}
