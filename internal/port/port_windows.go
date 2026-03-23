//go:build windows

package port

import "fmt"

var errUnsupported = fmt.Errorf("prt henüz Windows'u desteklemiyor. macOS veya Linux kullanın")

func GetListeningPorts() ([]ListeningPort, error) {
	return nil, errUnsupported
}

func FindByPort(portNum int) (*ListeningPort, error) {
	return nil, errUnsupported
}

func KillByPort(portNum int, force bool) (*ListeningPort, error) {
	return nil, errUnsupported
}
