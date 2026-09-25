package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

func newTestModel(ports ...models.Port) (*model, *[]int32) {
	var killed []int32
	m := &model{
		load:     func(context.Context) ([]models.Port, error) { return ports, nil },
		kill:     func(_ context.Context, pid int32) error { killed = append(killed, pid); return nil },
		interval: time.Second,
		ports:    ports,
	}
	return m, &killed
}

func key(m *model, k string) tea.Cmd {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	switch k {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	}
	_, cmd := m.Update(msg)
	return cmd
}

func port(n int, pid int32, name string) models.Port {
	return models.Port{Number: n, Address: "localhost", Protocol: "tcp", PID: pid, Process: &models.Process{PID: pid, Name: name}}
}

func TestNavigationAndDetail(t *testing.T) {
	m, _ := newTestModel(port(3000, 10, "node"), port(5432, 11, "postgres"))

	key(m, "j")
	key(m, "j") // clamps at last row
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.cursor)
	}
	key(m, "k")
	key(m, "k") // clamps at first row
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.cursor)
	}

	key(m, "enter")
	if m.mode != modeDetail || !strings.Contains(m.View(), "node (PID 10)") {
		t.Fatalf("detail view wrong: mode=%v\n%s", m.mode, m.View())
	}
	key(m, "esc")
	if m.mode != modeList {
		t.Fatal("esc should return to the list")
	}
}

func TestKillRequiresConfirmation(t *testing.T) {
	m, killed := newTestModel(port(3000, 10, "node"))

	// Anything but "y" cancels.
	key(m, "x")
	if m.mode != modeConfirmKill || !strings.Contains(m.View(), "[y/N]") {
		t.Fatal("x should ask for confirmation")
	}
	if cmd := key(m, "n"); cmd != nil || m.mode != modeList {
		t.Fatal("n should cancel without a command")
	}
	if len(*killed) != 0 {
		t.Fatalf("killed %v without confirmation", *killed)
	}

	key(m, "x")
	cmd := key(m, "y")
	if cmd == nil {
		t.Fatal("y should produce the kill command")
	}
	if msg, ok := cmd().(killedMsg); !ok || msg.err != nil {
		t.Fatalf("unexpected kill result %#v", msg)
	}
	if len(*killed) != 1 || (*killed)[0] != 10 {
		t.Fatalf("killed = %v, want [10]", *killed)
	}
}

func TestKillRefusedForContainerAndNoPID(t *testing.T) {
	c := port(8080, 20, "docker-proxy")
	c.Container = &models.Container{Name: "web"}
	m, killed := newTestModel(c, port(9000, 0, "-"))

	key(m, "x")
	if m.mode != modeList || !strings.Contains(m.notice, "container web") {
		t.Fatalf("container port: mode=%v notice=%q", m.mode, m.notice)
	}
	key(m, "j")
	key(m, "x")
	if m.mode != modeList || m.notice == "" {
		t.Fatalf("no-PID port: mode=%v notice=%q", m.mode, m.notice)
	}
	if len(*killed) != 0 {
		t.Fatal("must not kill")
	}
}

func TestRefreshKeepsCursorInBoundsAndShowsErrors(t *testing.T) {
	m, _ := newTestModel(port(1, 1, "a"), port(2, 2, "b"), port(3, 3, "c"))
	m.cursor = 2
	m.mode = modeDetail

	m.Update(loadedMsg{ports: []models.Port{port(1, 1, "a")}})
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want clamped to 0", m.cursor)
	}

	m.Update(loadedMsg{ports: nil})
	if m.mode != modeList || !strings.Contains(m.View(), "No listening ports") {
		t.Fatal("empty refresh should fall back to the list with an empty message")
	}

	m.Update(loadedMsg{err: errors.New("boom")})
	if !strings.Contains(m.View(), "error: boom") {
		t.Fatal("load error should be displayed")
	}
}

func TestStaleAndScrolling(t *testing.T) {
	var ports []models.Port
	for i := 0; i < 30; i++ {
		ports = append(ports, port(3000+i, int32(100+i), "proc"))
	}
	ports[29].Stale = true
	m, _ := newTestModel(ports...)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	m.cursor = 29

	view := m.View()
	if !strings.Contains(view, "3029") || !strings.Contains(view, "STALE") {
		t.Errorf("cursor row not visible/stale not shown:\n%s", view)
	}
	if strings.Contains(view, "3000 ") {
		t.Error("top rows should have scrolled out")
	}
}
