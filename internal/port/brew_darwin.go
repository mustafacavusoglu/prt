//go:build darwin

package port

import (
	"context"
	"os/exec"
	"time"
)

// brewServices tek bir `launchctl list` çağrısıyla tüm brew servislerini okur.
func brewServices() map[int]string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "launchctl", "list").Output()
	if err != nil {
		return nil
	}
	return parseLaunchctl(string(out))
}
