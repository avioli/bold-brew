package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"bbrew/internal/config"
	"bbrew/internal/models"
	"bbrew/internal/ui"
	"bbrew/internal/ui/theme"
)

// screenText returns the simulation screen as lines of text.
func screenText(screen tcell.SimulationScreen) string {
	cells, width, _ := screen.GetContents()
	var sb strings.Builder
	for i, c := range cells {
		if len(c.Runes) > 0 {
			sb.WriteRune(c.Runes[0])
		} else {
			sb.WriteRune(' ')
		}
		if (i+1)%width == 0 {
			sb.WriteRune('\n')
		}
	}
	return sb.String()
}

// readScreen reads the screen on the UI goroutine, after a draw.
func readScreen(app *tview.Application, screen tcell.SimulationScreen) string {
	var text string
	done := make(chan struct{})
	app.QueueUpdateDraw(func() {})
	app.QueueUpdate(func() { text = screenText(screen); close(done) })
	<-done
	return text
}

// waitFor polls the screen until it contains want.
func waitFor(t *testing.T, app *tview.Application, screen tcell.SimulationScreen, want string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		text := readScreen(app, screen)
		if strings.Contains(text, want) || time.Now().After(deadline) {
			if !strings.Contains(text, want) {
				t.Fatalf("screen lacks %q:\n%s", want, text)
			}
			return text
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// newSimUI runs a minimal bbrew UI on a 120x40 simulation screen.
func newSimUI(t *testing.T) (*AppService, *InputService, tcell.SimulationScreen) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	th := theme.NewTheme()
	s := &AppService{
		app: tview.NewApplication().SetScreen(screen), theme: th, layout: ui.NewLayout(th),
		packages: new([]models.Package), filteredPackages: new([]models.Package),
		brewfilePackages: new([]models.Package),
		vulnsService:     NewVulnsService(), diskUsage: NewDiskUsageService(),
		releaseAge: newTestReleaseAge(),
	}
	in := NewInputService(s, NewBrewService(), NewFlatpakService()).(*InputService)
	s.inputService = in
	s.layout.Setup()
	s.setupReleaseAge()
	s.app.SetInputCapture(in.HandleKeyEventInput).SetRoot(s.layout.Root(), true).
		SetFocus(s.layout.GetTable().View())
	s.app.SetBeforeDrawFunc(func(sc tcell.Screen) bool {
		w, h := sc.Size()
		s.layout.Resize(w, h)
		return false
	})
	go func() { _ = s.app.Run() }()
	t.Cleanup(s.app.Stop)
	waitFor(t, s.app, screen, "Help") // Run re-initialises the screen to 80x25
	screen.SetSize(120, 40)
	s.app.QueueUpdateDraw(func() {})

	return s, in, screen
}

func TestOptionsPanel(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	config.Set(config.Default())
	defer config.Set(config.Default())

	s, in, screen := newSimUI(t)
	_ = in

	key := func(k tcell.Key, r rune) {
		screen.InjectKey(k, r, tcell.ModNone)
	}

	key(tcell.KeyRune, 'o')
	text := waitFor(t, s.app, screen, "Options (Ctrl+S save, Esc cancel)")
	if !strings.Contains(text, "▸ View: start sort") {
		t.Fatalf("first row not marked:\n%s", text)
	}

	// Down moves to the next row.
	key(tcell.KeyDown, 0)
	waitFor(t, s.app, screen, "▸ View: start filter")

	// Move to "View: borders" and untick it; Q is not quit in the panel.
	key(tcell.KeyDown, 0)
	key(tcell.KeyDown, 0)
	waitFor(t, s.app, screen, "▸ View: borders")
	key(tcell.KeyRune, 'Q')
	key(tcell.KeyRune, ' ')
	text = waitFor(t, s.app, screen, "Save")
	if !strings.Contains(text, "Cancel") {
		t.Errorf("buttons not shown:\n%s", text)
	}
	key(tcell.KeyCtrlS, 0)
	waitFor(t, s.app, screen, "Settings saved to")

	if config.Get().View.Borders {
		t.Error("borders still on in the settings in effect")
	}
	saved, err := config.Load(config.Path())
	if err != nil || saved.View.Borders {
		t.Errorf("saved file: %+v, %v", saved.View, err)
	}
	if _, err := os.Stat(filepath.Join(cfgHome, "bbrew", "config.toml")); err != nil {
		t.Error(err)
	}

	// Esc cancels without saving.
	key(tcell.KeyRune, 'o')
	waitFor(t, s.app, screen, "Options (Ctrl+S save")
	key(tcell.KeyRune, ' ') // would change the start sort dropdown... opens it
	key(tcell.KeyEsc, 0)    // closes the dropdown list only
	waitFor(t, s.app, screen, "Options (Ctrl+S save")
	key(tcell.KeyEsc, 0) // closes the panel
	deadline := time.Now().Add(2 * time.Second)
	for strings.Contains(readScreen(s.app, screen), "Options (Ctrl+S") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if strings.Contains(readScreen(s.app, screen), "Options (Ctrl+S") {
		t.Error("Esc did not close the panel")
	}
}

func TestConfirmRecentRelease(t *testing.T) {
	config.Set(config.Default())
	defer config.Set(config.Default())
	s, in, screen := newSimUI(t)

	recent := models.NewPackageFromFormula(testFormula("jq", "1.8.1", 0, 0))
	old := models.NewPackageFromFormula(testFormula("wget", "1.25.0", 0, 0))
	s.releaseAge.entries[releaseKey(recent)] = releaseEntry{Time: time.Now().Add(-3 * time.Hour), Source: "bottle"}
	s.releaseAge.entries[releaseKey(old)] = releaseEntry{Time: time.Now().Add(-30 * 24 * time.Hour), Source: "bottle"}

	ran := make(chan string, 2)
	s.app.QueueUpdateDraw(func() {
		in.confirmRelease(recent, "plain question", "Update anyway?", func() { ran <- "recent" })
	})
	text := waitFor(t, s.app, screen, "Update anyway?")
	if !strings.Contains(text, "Released in the last 24h") || !strings.Contains(text, "jq 1.8.1, 3h ago") {
		t.Errorf("warning text:\n%s", text)
	}

	// With confirmations off, an old release runs at once; a recent one
	// still asks.
	c := config.Default()
	c.Actions.Confirm = false
	config.Set(c)
	s.app.QueueUpdateDraw(func() {
		in.closeModal()
		in.confirmRelease(old, "plain question", "Update anyway?", func() { ran <- "old" })
	})
	select {
	case got := <-ran:
		if got != "old" {
			t.Errorf("ran %s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("old release did not run without confirmation")
	}
	s.app.QueueUpdateDraw(func() {
		in.confirmRelease(recent, "plain question", "Update anyway?", func() { ran <- "recent" })
	})
	waitFor(t, s.app, screen, "Update anyway?")
	select {
	case got := <-ran:
		t.Errorf("%s ran without confirmation", got)
	default:
	}
}
