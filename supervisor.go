package grafsy

import (
	"net"
	"os"
)

// Selected supervisor
type supervisor string

// Report to supervisor state of the daemin
func (s supervisor) notify() {
	switch s {
	case "systemd":
		// TODO: Initialize address and connection in type instead of notify()
		socketAddr := &net.UnixAddr{
			Name: os.Getenv("NOTIFY_SOCKET"),
			Net:  "unixgram",
		}

		if socketAddr.Name == "" {
			return
		}

		conn, err := net.DialUnix(socketAddr.Net, nil, socketAddr)
		defer conn.Close()
		if err != nil {
			return
		}

		_, err = conn.Write([]byte("WATCHDOG=1"))
		if err != nil {
			return
		}
	}

}
