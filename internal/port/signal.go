package port

import (
	"fmt"
	"strings"
)

// Signal, bir sürece gönderilebilecek sinyallerin platformdan bağımsız kümesidir.
type Signal int

const (
	SigTERM Signal = iota
	SigKILL
	SigHUP
	SigINT
)

func (s Signal) String() string {
	switch s {
	case SigKILL:
		return "SIGKILL"
	case SigHUP:
		return "SIGHUP"
	case SigINT:
		return "SIGINT"
	default:
		return "SIGTERM"
	}
}

// ParseSignal "term", "SIGTERM", "kill", "hup", "int" gibi adları ayrıştırır.
func ParseSignal(s string) (Signal, error) {
	name := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(s)), "SIG")
	switch name {
	case "TERM":
		return SigTERM, nil
	case "KILL":
		return SigKILL, nil
	case "HUP":
		return SigHUP, nil
	case "INT":
		return SigINT, nil
	}
	return 0, fmt.Errorf("desteklenmeyen sinyal %q (TERM, KILL, HUP, INT)", s)
}
