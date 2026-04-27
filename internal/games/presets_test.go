package games

import (
	"strings"
	"testing"
)

func TestAll_NotEmpty(t *testing.T) {
	all := All()
	if len(all) < 9 {
		t.Errorf("expected ≥9 presets, got %d", len(all))
	}
}

func TestAll_ShapeIntegrity(t *testing.T) {
	for _, p := range All() {
		if strings.TrimSpace(p.Name) == "" {
			t.Errorf("preset has empty name: %+v", p)
		}
		if len(p.Ports) == 0 {
			t.Errorf("%s: no ports", p.Name)
		}
		for _, port := range p.Ports {
			if port.LocalPort == 0 {
				t.Errorf("%s: local port 0", p.Name)
			}
		}
	}
}

func TestAll_NoDuplicateNames(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range All() {
		if seen[p.Name] {
			t.Errorf("duplicate preset name: %s", p.Name)
		}
		seen[p.Name] = true
	}
}
