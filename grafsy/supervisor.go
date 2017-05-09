package grafsy

import (
	"net"
	"os"
)

// Selected supervisor
type Supervisor string

// Report to supervisor state of the daemin
func (s Supervisor) notify() {
	switch s {
	case "systemd":
		socketAddr := &net.UnixAddr{
			Name: os.Getenv("NOTIFY_SOCKET"),
			Net:  "unixgram",
		}

		if socketAddr.Name == "" {
			return
		}

		conn, err := net.DialUnix(socketAddr.Net, nil, socketAddr)
		if err != nil {
			return
		}

		_, err = conn.Write([]byte("WATCHDOG=1"))
		if err != nil {
			return
		}
	}

}
