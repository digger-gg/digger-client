// Package client is the digger agent: it dials the relay, registers
// tunnels, and proxies traffic between public peers and local services.
package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/digger-gg/digger-client/proto"
)

type ConnStatus int

const (
	StatusConnecting ConnStatus = iota
	StatusConnected
	StatusDisconnected
	StatusDenied
)

func (s ConnStatus) String() string {
	switch s {
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	case StatusDenied:
		return "denied"
	default:
		return "disconnected"
	}
}

type TunnelState int

const (
	TunnelPending TunnelState = iota
	TunnelOpen
	TunnelFailed
)

// Tunnel is one configured tunnel as seen by the user.
type Tunnel struct {
	Tid        uint32
	Proto      proto.Proto
	PublicPort uint16 // 0 = ask relay to auto-assign
	LocalAddr  string // host:port
	State      TunnelState
	Bound      string // public host:port the relay is listening on
	Slug       string // memorable name (e.g. "lucky-bear")
	Error      string
	Conns      uint32
}

type Snapshot struct {
	RelayAddr string
	RelayHost string // host portion of relay addr (used to format public address)
	Status    ConnStatus
	StatusMsg string
	Tunnels   []Tunnel
	Logs      []string
	BytesUp   uint64
	BytesDown uint64
}

// Command is a request from the UI to the client.
type Command interface{ isCommand() }

type CmdAddTunnel struct {
	Proto      proto.Proto
	PublicPort uint16
	LocalAddr  string
}

func (CmdAddTunnel) isCommand() {}

type CmdRemoveTunnel struct{ Tid uint32 }

func (CmdRemoveTunnel) isCommand() {}

type Config struct {
	Relay  string
	Secret string
	Name   string
}

const logLimit = 200

type Client struct {
	cfg Config

	mu        sync.Mutex
	status    ConnStatus
	statusMsg string
	tunnels   []*Tunnel
	logs      []string
	bytesUp   uint64
	bytesDown uint64
	nextTid   uint32

	cmds chan Command
}

func New(cfg Config) *Client {
	if cfg.Name == "" {
		h, _ := os.Hostname()
		cfg.Name = h
	}
	return &Client{
		cfg:     cfg,
		status:  StatusConnecting,
		nextTid: 1,
		cmds:    make(chan Command, 32),
	}
}

func (c *Client) Send(cmd Command) {
	select {
	case c.cmds <- cmd:
	default:
	}
}

func (c *Client) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	tt := make([]Tunnel, 0, len(c.tunnels))
	for _, t := range c.tunnels {
		tt = append(tt, *t)
	}
	logs := make([]string, len(c.logs))
	copy(logs, c.logs)
	host, _, _ := net.SplitHostPort(c.cfg.Relay)
	return Snapshot{
		RelayAddr: c.cfg.Relay,
		RelayHost: host,
		Status:    c.status,
		StatusMsg: c.statusMsg,
		Tunnels:   tt,
		Logs:      logs,
		BytesUp:   atomic.LoadUint64(&c.bytesUp),
		BytesDown: atomic.LoadUint64(&c.bytesDown),
	}
}

func (c *Client) log(format string, args ...any) {
	now := time.Now().Format("15:04:05")
	line := now + " " + fmt.Sprintf(format, args...)
	c.mu.Lock()
	c.logs = append(c.logs, line)
	if len(c.logs) > logLimit {
		c.logs = c.logs[len(c.logs)-logLimit:]
	}
	c.mu.Unlock()
}

func (c *Client) setStatus(s ConnStatus, msg string) {
	c.mu.Lock()
	c.status = s
	c.statusMsg = msg
	c.mu.Unlock()
}

// Run blocks, reconnecting on transient failure. Returns on auth denial or
// when ctx is cancelled.
func (c *Client) Run(ctx context.Context) error {
	for {
		c.setStatus(StatusConnecting, "connecting to "+c.cfg.Relay)
		c.log("connecting to %s", c.cfg.Relay)
		conn, err := net.DialTimeout("tcp", c.cfg.Relay, 10*time.Second)
		if err != nil {
			c.setStatus(StatusDisconnected, err.Error())
			c.log("connect failed: %v", err)
			if !sleepCtx(ctx, 3*time.Second) {
				return ctx.Err()
			}
			continue
		}
		err = c.runSession(ctx, conn)
		conn.Close()
		if err != nil {
			var denied *deniedErr
			if errors.As(err, &denied) {
				c.setStatus(StatusDenied, denied.reason)
				c.log("relay denied: %s", denied.reason)
				return err
			}
			c.setStatus(StatusDisconnected, err.Error())
			c.log("session ended: %v", err)
		} else {
			c.setStatus(StatusDisconnected, "closed")
			c.log("session ended")
		}
		if !sleepCtx(ctx, 3*time.Second) {
			return ctx.Err()
		}
	}
}

type deniedErr struct{ reason string }

func (d *deniedErr) Error() string { return "relay denied: " + d.reason }

func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}

func (c *Client) runSession(ctx context.Context, conn net.Conn) error {
	tcpConn, _ := conn.(*net.TCPConn)
	if tcpConn != nil {
		tcpConn.SetNoDelay(true)
	}

	var sec *string
	if c.cfg.Secret != "" {
		sec = &c.cfg.Secret
	}
	if err := c.write(conn, proto.ClientMsg{Hello: &proto.Hello{
		Version: proto.ProtoVersion, Secret: sec, Name: c.cfg.Name,
	}}); err != nil {
		return err
	}
	resp, err := c.read(conn)
	if err != nil {
		return err
	}
	switch {
	case resp.HelloOk != nil:
		// ok
	case resp.HelloDeny != nil:
		return &deniedErr{reason: resp.HelloDeny.Reason}
	default:
		return fmt.Errorf("unexpected reply to Hello")
	}
	c.setStatus(StatusConnected, "")
	c.log("connected")

	// Channels & state.
	out := make(chan proto.ClientMsg, 256)
	in := make(chan proto.ServerMsg, 256)
	errs := make(chan error, 2)

	// re-register tunnels and queue any pending commands.
	c.mu.Lock()
	for _, t := range c.tunnels {
		t.State = TunnelPending
		t.Bound = ""
		t.Slug = ""
		t.Error = ""
		t.Conns = 0
	}
	pending := make([]*Tunnel, len(c.tunnels))
	copy(pending, c.tunnels)
	c.mu.Unlock()
	for _, t := range pending {
		out <- proto.ClientMsg{OpenTunnel: &proto.OpenTunnel{
			Tid: t.Tid, Proto: t.Proto, PublicPort: t.PublicPort,
		}}
	}

	// reader goroutine
	go func() {
		for {
			m, err := c.read(conn)
			if err != nil {
				errs <- err
				close(in)
				return
			}
			in <- m
		}
	}()
	// writer goroutine
	go func() {
		for m := range out {
			if err := c.write(conn, m); err != nil {
				errs <- err
				return
			}
		}
	}()

	// per-(tid, peer) UDP sessions to the local game server.
	udpSessions := map[struct {
		tid  uint32
		peer string
	}]*net.UDPConn{}
	defer func() {
		for _, s := range udpSessions {
			s.Close()
		}
	}()

	// per-conn TCP local pumps: a channel feeding bytes into the local tcp socket.
	tcpLocal := map[uint64]chan []byte{}
	defer func() {
		for _, ch := range tcpLocal {
			close(ch)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			close(out)
			return nil
		case err := <-errs:
			close(out)
			return err
		case cmd := <-c.cmds:
			switch v := cmd.(type) {
			case CmdAddTunnel:
				c.mu.Lock()
				tid := c.nextTid
				c.nextTid++
				c.tunnels = append(c.tunnels, &Tunnel{
					Tid: tid, Proto: v.Proto, PublicPort: v.PublicPort,
					LocalAddr: v.LocalAddr, State: TunnelPending,
				})
				c.mu.Unlock()
				c.log("requesting %s :%d → %s", v.Proto, v.PublicPort, v.LocalAddr)
				out <- proto.ClientMsg{OpenTunnel: &proto.OpenTunnel{
					Tid: tid, Proto: v.Proto, PublicPort: v.PublicPort,
				}}
			case CmdRemoveTunnel:
				c.mu.Lock()
				removed := false
				for i, t := range c.tunnels {
					if t.Tid == v.Tid {
						c.tunnels = append(c.tunnels[:i], c.tunnels[i+1:]...)
						removed = true
						break
					}
				}
				c.mu.Unlock()
				if removed {
					c.log("removed tunnel %d", v.Tid)
					out <- proto.ClientMsg{CloseTunnel: &proto.CloseTunnel{Tid: v.Tid}}
				}
			}
		case m, ok := <-in:
			if !ok {
				close(out)
				return io.EOF
			}
			c.handleIn(m, out, tcpLocal, udpSessions)
		}
	}
}

func (c *Client) handleIn(
	m proto.ServerMsg,
	out chan<- proto.ClientMsg,
	tcpLocal map[uint64]chan []byte,
	udpSessions map[struct {
		tid  uint32
		peer string
	}]*net.UDPConn,
) {
	switch {
	case m.TunnelOk != nil:
		c.mu.Lock()
		for _, t := range c.tunnels {
			if t.Tid == m.TunnelOk.Tid {
				t.State = TunnelOpen
				t.Bound = m.TunnelOk.Bound
				t.Slug = m.TunnelOk.Slug
				_, port, _ := net.SplitHostPort(m.TunnelOk.Bound)
				if port != "" {
					var p uint16
					fmt.Sscanf(port, "%d", &p)
					t.PublicPort = p
				}
				break
			}
		}
		c.mu.Unlock()
		c.log("tunnel %d open on %s (%s)", m.TunnelOk.Tid, m.TunnelOk.Bound, m.TunnelOk.Slug)
	case m.TunnelDeny != nil:
		c.mu.Lock()
		for _, t := range c.tunnels {
			if t.Tid == m.TunnelDeny.Tid {
				t.State = TunnelFailed
				t.Error = m.TunnelDeny.Reason
				break
			}
		}
		c.mu.Unlock()
		c.log("tunnel %d denied: %s", m.TunnelDeny.Tid, m.TunnelDeny.Reason)
	case m.NewTcpConn != nil:
		c.handleNewTcp(*m.NewTcpConn, out, tcpLocal)
	case m.TcpData != nil:
		ch, ok := tcpLocal[m.TcpData.Conn]
		if ok {
			select {
			case ch <- m.TcpData.Data:
			default:
				// drop on slow consumer
			}
		}
	case m.TcpClose != nil:
		if ch, ok := tcpLocal[m.TcpClose.Conn]; ok {
			close(ch)
			delete(tcpLocal, m.TcpClose.Conn)
		}
	case m.UdpData != nil:
		c.handleInUdp(*m.UdpData, out, udpSessions)
	case m.Ping:
		out <- proto.ClientMsg{Pong: true}
	}
}

func (c *Client) handleNewTcp(n proto.NewTcpConn, out chan<- proto.ClientMsg, tcpLocal map[uint64]chan []byte) {
	c.mu.Lock()
	var local string
	for _, t := range c.tunnels {
		if t.Tid == n.Tid {
			local = t.LocalAddr
			t.Conns++
			break
		}
	}
	c.mu.Unlock()
	if local == "" {
		out <- proto.ClientMsg{TcpClose: &proto.TcpClose{Conn: n.Conn}}
		return
	}
	c.log("conn %d from %s → %s", n.Conn, n.Peer, local)
	ch := make(chan []byte, 64)
	tcpLocal[n.Conn] = ch
	go pumpLocalTcp(local, n.Conn, ch, out, c)
}

func pumpLocalTcp(local string, conn uint64, ch chan []byte, out chan<- proto.ClientMsg, c *Client) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	cnx, err := dialer.Dial("tcp", local)
	if err != nil {
		out <- proto.ClientMsg{TcpClose: &proto.TcpClose{Conn: conn}}
		c.log("conn %d dial %s failed: %v", conn, local, err)
		return
	}
	if t, ok := cnx.(*net.TCPConn); ok {
		t.SetNoDelay(true)
	}
	defer cnx.Close()

	// writer (relay→local)
	go func() {
		for data := range ch {
			if _, err := cnx.Write(data); err != nil {
				return
			}
		}
		if t, ok := cnx.(*net.TCPConn); ok {
			t.CloseWrite()
		}
	}()

	// reader (local→relay)
	buf := make([]byte, 16384)
	for {
		n, err := cnx.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			out <- proto.ClientMsg{TcpData: &proto.TcpData{Conn: conn, Data: data}}
			atomic.AddUint64(&c.bytesUp, uint64(n))
		}
		if err != nil {
			out <- proto.ClientMsg{TcpClose: &proto.TcpClose{Conn: conn}}
			return
		}
	}
}

func (c *Client) handleInUdp(
	u proto.UdpData,
	out chan<- proto.ClientMsg,
	sessions map[struct {
		tid  uint32
		peer string
	}]*net.UDPConn,
) {
	type key = struct {
		tid  uint32
		peer string
	}
	k := key{u.Tid, u.Peer}
	sock, ok := sessions[k]
	if !ok {
		c.mu.Lock()
		var local string
		for _, t := range c.tunnels {
			if t.Tid == u.Tid {
				local = t.LocalAddr
				break
			}
		}
		c.mu.Unlock()
		if local == "" {
			return
		}
		laddr, err := net.ResolveUDPAddr("udp", local)
		if err != nil {
			return
		}
		s, err := net.DialUDP("udp", nil, laddr)
		if err != nil {
			return
		}
		sessions[k] = s
		sock = s
		go func() {
			buf := make([]byte, 65536)
			for {
				n, err := s.Read(buf)
				if err != nil {
					return
				}
				data := make([]byte, n)
				copy(data, buf[:n])
				out <- proto.ClientMsg{UdpData: &proto.UdpData{
					Tid: u.Tid, Peer: u.Peer, Data: data,
				}}
				atomic.AddUint64(&c.bytesUp, uint64(n))
			}
		}()
	}
	if _, err := sock.Write(u.Data); err == nil {
		atomic.AddUint64(&c.bytesDown, uint64(len(u.Data)))
	}
}

func (c *Client) write(w io.Writer, m proto.ClientMsg) error {
	err := proto.WriteMsg(w, m)
	if err == nil {
		// approximate; we don't have a sniffing writer here
	}
	return err
}

func (c *Client) read(r io.Reader) (proto.ServerMsg, error) {
	return proto.ReadMsg(r)
}
