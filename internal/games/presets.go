// Package games defines tunnel presets for popular game servers.
package games

import "github.com/digger-gg/digger-client/proto"

// PortSpec is one (proto, port) pair used by a game.
type PortSpec struct {
	Proto      proto.Proto
	LocalPort  uint16 // the port the game server listens on locally
	PublicPort uint16 // 0 = let the relay auto-assign
}

// Preset is a named bundle of port specs for a game.
type Preset struct {
	Name  string
	Note  string // short description shown next to name
	Ports []PortSpec
}

func All() []Preset {
	return []Preset{
		{
			Name: "Minecraft (Java)",
			Note: "vanilla / modded — TCP 25565",
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
			Note: "UDP 26900-26902 + TCP 26900",
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
			Note: "UDP 7777 + 7778 + 27015",
			Ports: []PortSpec{
				{Proto: proto.Udp, LocalPort: 7777},
				{Proto: proto.Udp, LocalPort: 7778},
				{Proto: proto.Udp, LocalPort: 27015},
			},
		},
		{
			Name: "CS / Source",
			Note: "UDP 27015 + TCP 27015",
			Ports: []PortSpec{
				{Proto: proto.Tcp, LocalPort: 27015},
				{Proto: proto.Udp, LocalPort: 27015},
			},
		},
	}
}
