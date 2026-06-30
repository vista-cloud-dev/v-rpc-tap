// Package broker is a minimal RPC Broker [XWB] TCP *client* session: it completes the
// TCPConnect handshake and then fires RPCs, so the live L-block tests (and, later, the
// load rig) can drive real traffic through MAIN->CALLP^XWBPRS — the point where the tap
// is spliced. A bare RPC chunk with no handshake is rejected at NEW^XWBTCPM (:137) and
// never reaches CALLP, so a real session is required.
//
// This is the v rpc-tap domain acting as a broker client (the CPRS-equivalent, like
// v-rpc-debug's ping) — the legitimate TCP-client exception to the engine-access rule,
// NOT the m-driver-sdk engine seam. It takes a broker host:port, not a driver.
package broker

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"github.com/vista-cloud-dev/v-rpc-tap/internal/xwbwire"
)

// Session is one connected broker session (one broker job, one $ETRAP frame).
type Session struct {
	conn    net.Conn
	timeout time.Duration
}

// Dial opens a TCP connection and completes the [XWB] TCPConnect handshake. It returns
// an error if the broker rejects the connection (i.e. the response is not "accept").
func Dial(addr string, timeout time.Duration) (*Session, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	s := &Session{conn: conn, timeout: timeout}
	resp, err := s.roundtrip(xwbwire.Connect())
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("handshake: %w", err)
	}
	if !bytes.Contains(resp, []byte("accept")) {
		_ = conn.Close()
		return nil, fmt.Errorf("broker rejected handshake: %q", resp)
	}
	return s, nil
}

// Fire sends one no-argument RPC and returns the broker's full response bytes
// (EOT-terminated). The reply reaches CALLP^XWBPRS via the MAIN loop, so on a spliced
// broker req^/rsp^VSLRTAP fire as a side effect.
func (s *Session) Fire(name string) ([]byte, error) {
	return s.roundtrip(xwbwire.RPCMessage(name))
}

// FireRaw sends arbitrary pre-built [XWB] message bytes and returns the response. Used
// to drive fault-injecting messages (malformed param chunks) for the L2 live suite.
func (s *Session) FireRaw(msg []byte) ([]byte, error) {
	return s.roundtrip(msg)
}

// Close sends #BYE# so the broker tears its job down normally, then closes the socket.
func (s *Session) Close() error {
	_, _ = s.roundtrip(xwbwire.Bye()) // best effort: the server replies then drops us
	return s.conn.Close()
}

// roundtrip writes one message and reads the EOT-terminated response under the timeout.
func (s *Session) roundtrip(msg []byte) ([]byte, error) {
	if err := s.conn.SetWriteDeadline(time.Now().Add(s.timeout)); err != nil {
		return nil, err
	}
	if _, err := s.conn.Write(msg); err != nil {
		return nil, err
	}
	return readEOT(s.conn, s.timeout)
}

// readEOT reads until the broker's 0x04 EOT response terminator (SND^XWBRW ends every
// response with WRITE($C(4))) or the deadline. It returns whatever was read alongside a
// read error, so a connection the server drops after the reply (e.g. post-#BYE#) still
// yields the response bytes.
func readEOT(conn net.Conn, timeout time.Duration) ([]byte, error) {
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	var buf []byte
	tmp := make([]byte, 1024)
	for {
		n, err := conn.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if bytes.IndexByte(tmp[:n], 0x04) >= 0 {
			return buf, nil
		}
		if err != nil {
			return buf, err
		}
	}
}
