package zencached

import (
	"net"
	"testing"
	"time"
)

// TestDialTCPHonorsHostConnectionTimeout - a dial to an address that never answers must not block past the timeout
func TestDialTCPHonorsHostConnectionTimeout(t *testing.T) {

	timeout := 500 * time.Millisecond

	telnet := &Telnet{configuration: TelnetConfiguration{HostConnectionTimeout: timeout}}

	// 192.0.2.0/24 (TEST-NET-1) is reserved for documentation and never routed
	address := &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 11211}

	start := time.Now()

	connection, err := telnet.dialTCP(address)
	if err == nil {
		connection.Close()
		t.Fatal("expected an error dialing an unroutable address")
	}

	if elapsed := time.Since(start); elapsed > 2*timeout {
		t.Fatalf("dial took %s, expected to give up after about %s", elapsed, timeout)
	}
}

func TestDialTCPConnects(t *testing.T) {

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	telnet := &Telnet{configuration: TelnetConfiguration{HostConnectionTimeout: time.Second}}

	connection, err := telnet.dialTCP(listener.Addr().(*net.TCPAddr))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	connection.Close()
}
