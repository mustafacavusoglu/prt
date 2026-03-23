package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "prt",
	Short: "Açık portları listele ve kapat",
	Long:  "prt - Açıkta unutulan servisleri (Redis, Postgres vb.) hızlıca bulup kapatmak için basit bir CLI aracı.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
