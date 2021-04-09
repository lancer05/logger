package logger

import (
	"net"
	"strings"
)

func parseIP(remoteAddr string) string {
	if strings.HasPrefix(remoteAddr, "[") || strings.Count(remoteAddr, ":") == 1 {
		ip, _, _ := net.SplitHostPort(remoteAddr)
		return ip
	}

	return remoteAddr
}
