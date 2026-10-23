package port

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// brewLabelPrefixes brew'in launchd etiket önekleridir: Homebrew 7+ "sh.brew.<formül>",
// eski sürümler "homebrew.mxcl.<formül>" kullanır. Formül adı "postgresql@17" gibi sürümlü olabilir.
var brewLabelPrefixes = []string{"sh.brew.", "homebrew.mxcl."}

// parseLaunchctl, `launchctl list` çıktısından PID -> brew servis adı haritası çıkarır.
// Satır biçimi: "PID<TAB>Status<TAB>Label"; çalışmayan servislerde PID "-" olur.
func parseLaunchctl(out string) map[int]string {
	m := make(map[int]string)
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) != 3 {
			continue
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		for _, prefix := range brewLabelPrefixes {
			if name, ok := strings.CutPrefix(f[2], prefix); ok && name != "" {
				m[pid] = name
				break
			}
		}
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
