// Tiny interop check: connect, hello, request auto-assigned TCP tunnel, print response.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/digger-gg/digger-client/proto"
)

func main() {
	relay := flag.String("relay", "127.0.0.1:7777", "relay address")
	secret := flag.String("secret", "", "shared secret")
	pubport := flag.Int("port", 0, "public port (0 = auto)")
	flag.Parse()

	conn, err := net.Dial("tcp", *relay)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	host, _ := os.Hostname()
	var sec *string
	if *secret != "" {
		sec = secret
	}

	// Hello
	if err := proto.WriteMsg(conn, proto.ClientMsg{Hello: &proto.Hello{
		Version: proto.ProtoVersion, Secret: sec, Name: "iotest@" + host,
	}}); err != nil {
		log.Fatalf("write hello: %v", err)
	}
	resp, err := proto.ReadMsg(conn)
	if err != nil {
		log.Fatalf("read hello reply: %v", err)
	}
	switch {
	case resp.HelloOk != nil:
		fmt.Printf("HelloOk version=%d\n", resp.HelloOk.Version)
	case resp.HelloDeny != nil:
		log.Fatalf("HelloDeny: %s", resp.HelloDeny.Reason)
	default:
		log.Fatalf("unexpected reply to Hello: %+v", resp)
	}

	// OpenTunnel
	if err := proto.WriteMsg(conn, proto.ClientMsg{OpenTunnel: &proto.OpenTunnel{
		Tid: 1, Proto: proto.Tcp, PublicPort: uint16(*pubport),
	}}); err != nil {
		log.Fatalf("write open: %v", err)
	}
	resp, err = proto.ReadMsg(conn)
	if err != nil {
		log.Fatalf("read open reply: %v", err)
	}
	switch {
	case resp.TunnelOk != nil:
		fmt.Printf("TunnelOk tid=%d bound=%s slug=%s\n", resp.TunnelOk.Tid, resp.TunnelOk.Bound, resp.TunnelOk.Slug)
	case resp.TunnelDeny != nil:
		log.Fatalf("TunnelDeny: %s", resp.TunnelDeny.Reason)
	default:
		log.Fatalf("unexpected reply: %+v", resp)
	}
	fmt.Println("interop OK")
}
