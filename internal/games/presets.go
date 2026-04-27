// Package games defines tunnel presets for popular game servers.
package games

import "github.com/digger-gg/digger-client/proto"

// PortSpec is one (proto, port) pair used by a game.
type PortSpec struct {
	Proto      proto.Proto
	LocalPort  uint16 // port the game server listens on locally
	PublicPort uint16 // 0 = let the relay auto-assign
}

// Preset is a named bundle of port specs for a game.
type Preset struct {
	Name  string
	Note  string
	Ports []PortSpec
}

func All() []Preset {
	return []Preset{
		{
			Name: "Minecraft (Java)",
			Note: "TCP 25565",
			Ports: []PortSpec{
				{Proto: proto.Tcp, LocalPort: 25565},
			},
		},
		{
			Name: "Minecraft (Bedrock)",
			Note: "UDP 19132",
			Ports: []PortSpec{
				{Proto: proto.Udp, LocalPort: 19132},
			},
		},
		{
			Name: "Valheim",
			Note: "UDP 2456-2458",
			Ports: []PortSpec{
				{Proto: proto.Udp, LocalPort: 2456},
				{Proto: proto.Udp, LocalPort: 2457},
				{Proto: proto.Udp, LocalPort: 2458},
			},
		},
		{
			Name: "Terraria",
			Note: "TCP 7777",
			Ports: []PortSpec{
				{Proto: proto.Tcp, LocalPort: 7777},
			},
		},
		{
			Name: "Factorio",
			Note: "UDP 34197",
			Ports: []PortSpec{
				{Proto: proto.Udp, LocalPort: 34197},
			},
		},
		{
			Name: "7 Days to Die",
			Note: "TCP 26900 + UDP 26900-26902",
			Ports: []PortSpec{
				{Proto: proto.Tcp, LocalPort: 26900},
				{Proto: proto.Udp, LocalPort: 26900},
				{Proto: proto.Udp, LocalPort: 26901},
				{Proto: proto.Udp, LocalPort: 26902},
			},
		},
		{
			Name: "Project Zomboid",
			Note: "UDP 16261 + 16262",
			Ports: []PortSpec{
				{Proto: proto.Udp, LocalPort: 16261},
				{Proto: proto.Udp, LocalPort: 16262},
			},
		},
		{
			Name: "ARK: Survival",
			Note: "UDP 7777, 7778, 27015",
			Ports: []PortSpec{
				{Proto: proto.Udp, LocalPort: 7777},
				{Proto: proto.Udp, LocalPort: 7778},
				{Proto: proto.Udp, LocalPort: 27015},
			},
		},
		{
			Name: "CS / Source",
			Note: "TCP + UDP 27015",
			Ports: []PortSpec{
				{Proto: proto.Tcp, LocalPort: 27015},
				{Proto: proto.Udp, LocalPort: 27015},
			},
		},
		{
			Name: "Rust",
			Note: "UDP 28015 + RCON TCP 28016",
			Ports: []PortSpec{
				{Proto: proto.Udp, LocalPort: 28015},
				{Proto: proto.Tcp, LocalPort: 28016},
			},
		},
		{
			Name: "Satisfactory",
			Note: "UDP 7777",
			Ports: []PortSpec{
				{Proto: proto.Udp, LocalPort: 7777},
			},
		},
		{
			Name: "V Rising",
			Note: "UDP 9876 + 9877",
			Ports: []PortSpec{
				{Proto: proto.Udp, LocalPort: 9876},
				{Proto: proto.Udp, LocalPort: 9877},
			},
		},
		{
			Name: "Don't Starve Together",
			Note: "UDP 10999 + 11000",
			Ports: []PortSpec{
				{Proto: proto.Udp, LocalPort: 10999},
				{Proto: proto.Udp, LocalPort: 11000},
			},
		},
		{
			Name: "Garry's Mod",
			Note: "TCP + UDP 27015",
			Ports: []PortSpec{
				{Proto: proto.Tcp, LocalPort: 27015},
				{Proto: proto.Udp, LocalPort: 27015},
			},
		},
		{
			Name: "Unturned",
			Note: "UDP 27015-27017",
			Ports: []PortSpec{
				{Proto: proto.Udp, LocalPort: 27015},
				{Proto: proto.Udp, LocalPort: 27016},
				{Proto: proto.Udp, LocalPort: 27017},
			},
		},
	}
}
