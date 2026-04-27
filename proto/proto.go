// Package proto mirrors the Rust relay's wire format:
// length-prefixed (u32 BE) MessagePack with adjacently-tagged enums
// ({"t": "VariantName", "c": <payload>}).
package proto

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/vmihailenco/msgpack/v5"
)

const (
	ProtoVersion uint32 = 1
	MaxFrame            = 1 << 20
)

// Proto identifies a transport protocol. Encodes as the bare string
// "Tcp" or "Udp" to match Rust's default enum encoding.
type Proto string

const (
	Tcp Proto = "Tcp"
	Udp Proto = "Udp"
)

// ─── client → relay payloads ────────────────────────────────────────────

type Hello struct {
	Version uint32  `msgpack:"version"`
	Secret  *string `msgpack:"secret"`
	Name    string  `msgpack:"name"`
	Token   *string `msgpack:"token,omitempty"` // Firebase ID token from `digger login`
}

type OpenTunnel struct {
	Tid        uint32 `msgpack:"tid"`
	Proto      Proto  `msgpack:"proto"`
	PublicPort uint16 `msgpack:"public_port"` // 0 = auto-assign
}

type CloseTunnel struct {
	Tid uint32 `msgpack:"tid"`
}

type TcpData struct {
	Conn uint64 `msgpack:"conn"`
	Data []byte `msgpack:"data"`
}

type TcpClose struct {
	Conn uint64 `msgpack:"conn"`
}

type UdpData struct {
	Tid  uint32 `msgpack:"tid"`
	Peer string `msgpack:"peer"`
	Data []byte `msgpack:"data"`
}

// ─── relay → client payloads ────────────────────────────────────────────

type HelloOk struct {
	Version uint32 `msgpack:"version"`
}

type HelloDeny struct {
	Reason string `msgpack:"reason"`
}

type TunnelOk struct {
	Tid   uint32 `msgpack:"tid"`
	Bound string `msgpack:"bound"`
	Slug  string `msgpack:"slug"`
}

type TunnelDeny struct {
	Tid    uint32 `msgpack:"tid"`
	Reason string `msgpack:"reason"`
}

type NewTcpConn struct {
	Conn uint64 `msgpack:"conn"`
	Tid  uint32 `msgpack:"tid"`
	Peer string `msgpack:"peer"`
}

// ─── tagged frame ───────────────────────────────────────────────────────

// frame is the on-the-wire shape: {"t": "<variant>", "c": <payload>}
// Variants without payload omit the "c" field.
type frame struct {
	T string             `msgpack:"t"`
	C msgpack.RawMessage `msgpack:"c,omitempty"`
}

// ClientMsg is a tagged sum type. Exactly one field is non-nil.
type ClientMsg struct {
	Hello       *Hello
	OpenTunnel  *OpenTunnel
	CloseTunnel *CloseTunnel
	TcpData     *TcpData
	TcpClose    *TcpClose
	UdpData     *UdpData
	Pong        bool
}

func (m ClientMsg) marshal() ([]byte, error) {
	var f frame
	var payload any
	switch {
	case m.Hello != nil:
		f.T, payload = "Hello", m.Hello
	case m.OpenTunnel != nil:
		f.T, payload = "OpenTunnel", m.OpenTunnel
	case m.CloseTunnel != nil:
		f.T, payload = "CloseTunnel", m.CloseTunnel
	case m.TcpData != nil:
		f.T, payload = "TcpData", m.TcpData
	case m.TcpClose != nil:
		f.T, payload = "TcpClose", m.TcpClose
	case m.UdpData != nil:
		f.T, payload = "UdpData", m.UdpData
	case m.Pong:
		f.T = "Pong"
	default:
		return nil, fmt.Errorf("ClientMsg: empty")
	}
	if payload != nil {
		raw, err := msgpack.Marshal(payload)
		if err != nil {
			return nil, err
		}
		f.C = raw
	}
	return msgpack.Marshal(&f)
}

// ServerMsg is a tagged sum type from the relay.
type ServerMsg struct {
	HelloOk    *HelloOk
	HelloDeny  *HelloDeny
	TunnelOk   *TunnelOk
	TunnelDeny *TunnelDeny
	NewTcpConn *NewTcpConn
	TcpData    *TcpData
	TcpClose   *TcpClose
	UdpData    *UdpData
	Ping       bool
}

func (m *ServerMsg) unmarshal(b []byte) error {
	var f frame
	if err := msgpack.Unmarshal(b, &f); err != nil {
		return err
	}
	switch f.T {
	case "HelloOk":
		m.HelloOk = &HelloOk{}
		return msgpack.Unmarshal(f.C, m.HelloOk)
	case "HelloDeny":
		m.HelloDeny = &HelloDeny{}
		return msgpack.Unmarshal(f.C, m.HelloDeny)
	case "TunnelOk":
		m.TunnelOk = &TunnelOk{}
		return msgpack.Unmarshal(f.C, m.TunnelOk)
	case "TunnelDeny":
		m.TunnelDeny = &TunnelDeny{}
		return msgpack.Unmarshal(f.C, m.TunnelDeny)
	case "NewTcpConn":
		m.NewTcpConn = &NewTcpConn{}
		return msgpack.Unmarshal(f.C, m.NewTcpConn)
	case "TcpData":
		m.TcpData = &TcpData{}
		return msgpack.Unmarshal(f.C, m.TcpData)
	case "TcpClose":
		m.TcpClose = &TcpClose{}
		return msgpack.Unmarshal(f.C, m.TcpClose)
	case "UdpData":
		m.UdpData = &UdpData{}
		return msgpack.Unmarshal(f.C, m.UdpData)
	case "Ping":
		m.Ping = true
		return nil
	default:
		return fmt.Errorf("unknown ServerMsg variant %q", f.T)
	}
}

// ─── framing ────────────────────────────────────────────────────────────

func WriteMsg(w io.Writer, m ClientMsg) error {
	body, err := m.marshal()
	if err != nil {
		return err
	}
	if len(body) > MaxFrame {
		return fmt.Errorf("frame too large: %d", len(body))
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

func ReadMsg(r io.Reader) (ServerMsg, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return ServerMsg{}, err
	}
	length := binary.BigEndian.Uint32(hdr[:])
	if length > MaxFrame {
		return ServerMsg{}, fmt.Errorf("frame too large: %d", length)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return ServerMsg{}, err
	}
	var m ServerMsg
	if err := m.unmarshal(body); err != nil {
		return ServerMsg{}, err
	}
	return m, nil
}
