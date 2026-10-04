package cmd

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version GoReleaser/Makefile tarafından -ldflags "-X .../cmd.version=..." ile doldurulur.
var version = "dev"

// Çıkış kodları: betiklerde hata türünü ayırt etmek için.
const (
	exitGeneric    = 1 // beklenmeyen hata
	exitNotFound   = 2 // port üzerinde dinleyen süreç yok
	exitPermission = 3 // yetki yok (sudo gerekebilir)
	exitCancelled  = 4 // kullanıcı onayı vermedi
	exitSurvived   = 5 // sinyal gönderildi ama süreç kapanmadı
)

// exitError belirli bir çıkış koduyla biten hatadır. err nil ise mesaj basılmaz.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error { return e.err }

func fail(code int, format string, a ...any) error {
	return &exitError{code: code, err: fmt.Errorf(format, a...)}
}

var noColor bool

var rootCmd = &cobra.Command{
	Use:   "prt",
	Short: "Açık portları listele ve kapat",
	Long: `prt - Açıkta unutulan servisleri (Redis, Postgres vb.) hızlıca bulup kapatmak için basit bir CLI aracı.

Argümansız ve bir terminalde çalıştırılırsa interaktif moda girer: portları
numaralı listeler, kapatmak istediklerinizi seçtirir.`,
	Version:       buildVersion(),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
			return runInteractive(cmd, newKillOptions(), false, false)
		}
		return cmd.Help()
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "renkli çıktıyı kapat (NO_COLOR ortam değişkeni de desteklenir)")
	rootCmd.SetVersionTemplate("prt {{.Version}}\n")
}

// Execute komutu çalıştırır ve hata varsa uygun çıkış koduyla programı sonlandırır.
func Execute() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}
	code := exitGeneric
	var ee *exitError
	if errors.As(err, &ee) {
		code = ee.code
	}
	if msg := err.Error(); msg != "" {
		fmt.Fprintln(os.Stderr, "Hata:", msg)
		if code == exitGeneric && !errors.As(err, &ee) {
			fmt.Fprintln(os.Stderr, "Yardım için: prt --help")
		}
	}
	os.Exit(code)
}

func buildVersion() string {
	if version != "dev" {
		return version
	}
	// `go install github.com/.../prt@vX.Y.Z` ile kurulduğunda sürüm modül bilgisinde bulunur.
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}
