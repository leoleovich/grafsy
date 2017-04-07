package main

import (
	"net"
	"os"
)

type Supervisor struct {
	name string
}

// I want to make this function universal in case we support other daemonization in the future
func (s Supervisor) notify() {
	switch s.name {
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
