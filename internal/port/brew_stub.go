//go:build !darwin

package port

// brewServices yalnızca macOS'ta (launchctl) anlamlıdır.
func brewServices() map[int]string { return nil }
