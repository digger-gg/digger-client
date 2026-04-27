package proto

import "github.com/vmihailenco/msgpack/v5"

// unmarshalForTest mirrors ServerMsg.unmarshal but for ClientMsg variants.
// Test-only — exposes decoding so we can roundtrip ClientMsg in tests.
func (m *ClientMsg) unmarshalForTest(b []byte) error {
	var f frame
	if err := msgpack.Unmarshal(b, &f); err != nil {
		return err
	}
	switch f.T {
	case "Hello":
		m.Hello = &Hello{}
		return msgpack.Unmarshal(f.C, m.Hello)
	case "OpenTunnel":
		m.OpenTunnel = &OpenTunnel{}
		return msgpack.Unmarshal(f.C, m.OpenTunnel)
	case "CloseTunnel":
		m.CloseTunnel = &CloseTunnel{}
		return msgpack.Unmarshal(f.C, m.CloseTunnel)
	case "TcpData":
		m.TcpData = &TcpData{}
		return msgpack.Unmarshal(f.C, m.TcpData)
	case "TcpClose":
		m.TcpClose = &TcpClose{}
		return msgpack.Unmarshal(f.C, m.TcpClose)
	case "UdpData":
		m.UdpData = &UdpData{}
		return msgpack.Unmarshal(f.C, m.UdpData)
	case "Pong":
		m.Pong = true
		return nil
	}
	return nil
}
