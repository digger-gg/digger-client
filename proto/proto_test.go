package proto

import (
	"bytes"
	"reflect"
	"testing"
)

// roundTripClient encodes a ClientMsg, decodes a ServerMsg back from the
// same bytes via the framing layer, and checks the variant survives.
// Since framing is symmetric (ClientMsg and ServerMsg share a tag/content
// shape), we can roundtrip ClientMsg → ClientMsg via the framing.

func TestRoundTrip_ClientMsg(t *testing.T) {
	cases := []ClientMsg{
		{Hello: &Hello{Version: ProtoVersion, Name: "ando"}},
		{Hello: &Hello{Version: ProtoVersion, Secret: ptr("s"), Name: "ando"}},
		{OpenTunnel: &OpenTunnel{Tid: 1, Proto: Tcp, PublicPort: 0}},
		{OpenTunnel: &OpenTunnel{Tid: 7, Proto: Udp, PublicPort: 25565}},
		{CloseTunnel: &CloseTunnel{Tid: 3}},
		{TcpData: &TcpData{Conn: 9, Data: []byte("hello")}},
		{TcpClose: &TcpClose{Conn: 9}},
		{UdpData: &UdpData{Tid: 2, Peer: "1.2.3.4:5", Data: []byte{0xff, 0x00}}},
		{Pong: true},
	}

	for _, in := range cases {
		t.Run(variantName(in), func(t *testing.T) {
			body, err := in.marshal()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var out ClientMsg
			if err := out.unmarshalForTest(body); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !reflect.DeepEqual(in, out) {
				t.Errorf("round-trip mismatch:\n  in:  %+v\n  out: %+v", in, out)
			}
		})
	}
}

func TestFraming_LengthPrefix(t *testing.T) {
	var buf bytes.Buffer
	in := ClientMsg{TcpData: &TcpData{Conn: 1, Data: bytes.Repeat([]byte{0xab}, 4096)}}
	if err := WriteMsg(&buf, in); err != nil {
		t.Fatalf("write: %v", err)
	}
	// expect 4-byte big-endian length prefix
	if buf.Len() < 4 {
		t.Fatalf("frame too short: %d", buf.Len())
	}
	prefix := uint32(buf.Bytes()[0])<<24 |
		uint32(buf.Bytes()[1])<<16 |
		uint32(buf.Bytes()[2])<<8 |
		uint32(buf.Bytes()[3])
	if int(prefix) != buf.Len()-4 {
		t.Errorf("length prefix %d disagrees with body len %d", prefix, buf.Len()-4)
	}
}

func variantName(m ClientMsg) string {
	switch {
	case m.Hello != nil:
		return "Hello"
	case m.OpenTunnel != nil:
		return "OpenTunnel"
	case m.CloseTunnel != nil:
		return "CloseTunnel"
	case m.TcpData != nil:
		return "TcpData"
	case m.TcpClose != nil:
		return "TcpClose"
	case m.UdpData != nil:
		return "UdpData"
	case m.Pong:
		return "Pong"
	}
	return "Empty"
}

func ptr(s string) *string { return &s }
