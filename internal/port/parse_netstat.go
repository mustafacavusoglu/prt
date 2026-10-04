package port

import (
	"encoding/csv"
	"strconv"
	"strings"
)

// parseNetstat Windows `netstat -ano -p <proto>` çıktısını ayrıştırır.
//
// TCP:  Proto Yerel Uzak  Durum    PID   (yalnızca LISTENING / DINLENIYOR alınır)
// UDP:  Proto Yerel Uzak  PID
func parseNetstat(data, proto string) []Listener {
	var out []Listener
	for _, line := range strings.Split(data, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 || !strings.EqualFold(f[0], proto) {
			continue
		}
		var pidStr string
		switch proto {
		case "tcp":
			if len(f) < 5 || !isListeningState(f[3]) {
				continue
			}
			pidStr = f[4]
		case "udp":
			pidStr = f[len(f)-1]
		}
		host, port, ok := splitHostPort(f[1])
		if !ok {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		out = append(out, Listener{
			PID: pid, User: "-", UID: -1, Port: port, Proto: proto, Addresses: []string{host},
		})
	}
	return out
}

// isListeningState netstat'ın yerelleştirilmiş durum adlarını tanır.
func isListeningState(s string) bool {
	switch strings.ToUpper(s) {
	case "LISTENING", "DINLENIYOR", "DİNLENİYOR", "ECOUTE", "ABHÖREN", "ESCUCHANDO":
		return true
	}
	return false
}

// parseTasklist `tasklist /FO CSV /NH` çıktısından PID -> görüntü adı haritası çıkarır.
func parseTasklist(data string) map[int]string {
	m := make(map[int]string)
	r := csv.NewReader(strings.NewReader(data))
	r.FieldsPerRecord = -1
	recs, _ := r.ReadAll()
	for _, rec := range recs {
		if len(rec) < 2 {
			continue
		}
		if pid, err := strconv.Atoi(rec[1]); err == nil {
			m[pid] = rec[0]
		}
	}
	return m
}
