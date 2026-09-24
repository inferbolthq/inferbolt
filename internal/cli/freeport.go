package cli

import (
	"fmt"
	"net"
)

// FreeTCPPort finds an unused local port by binding to :0 and releasing it.
func FreeTCPPort() (int, error) {
	//nolint:noctx // binding to :0 returns immediately; there is nothing to cancel
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close() //nolint:errcheck // releasing the probe socket; nothing to recover

	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected address type %T from tcp listener", l.Addr())
	}
	return addr.Port, nil
}
