package port

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const brewLabelPrefix = "homebrew.mxcl."

// parseLaunchctl, `launchctl list` çıktısından PID -> brew servis adı haritası çıkarır.
// Satır biçimi: "PID<TAB>Status<TAB>Label"; çalışmayan servislerde PID "-" olur.
func parseLaunchctl(out string) map[int]string {
	m := make(map[int]string)
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) != 3 || !strings.HasPrefix(f[2], brewLabelPrefix) {
			continue
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		m[pid] = strings.TrimPrefix(f[2], brewLabelPrefix)
	}
	return m
}

// StopBrewService `brew services stop <name>` komutunu çalıştırır.
func StopBrewService(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "brew", "services", "stop", name).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
