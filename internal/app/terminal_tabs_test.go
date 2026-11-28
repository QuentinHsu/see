package app

import (
	"testing"

	"see/locales"
)

func termTabsHarness(t *testing.T) (*harness, *termTab, *termTab) {
	t.Helper()
	h := newHarness(t, writeTree(t, map[string]string{"a.txt": "text"}))
	h.open("a.txt")
	p1 := &termPane{id: 1, title: "zsh"}
	p2 := &termPane{id: 2, title: "zsh"}
	t1 := &termTab{id: 1, root: &termNode{id: 1, pane: p1}, active: p1}
	t2 := &termTab{id: 2, name: "build server", root: &termNode{id: 2, pane: p2}, active: p2}
	h.a.term = termPanel{open: true, view: panelTerminal, tabs: []*termTab{t1, t2}, active: t1, focus: p1}
	return h, t1, t2
}

// The terminals are tabs of their own, in the row under the views, and
// a click shows one.
func TestTerminalTabsBarSelectsTabs(t *testing.T) {
	h, t1, t2 := termTabsHarness(t)
	h.tt.Frame()
	if !h.tt.HasText("build server") {
		t.Fatalf("terminal tab missing: %v", h.tt.Texts())
	}
	if err := h.tt.Click("build server"); err != nil {
		t.Fatal(err)
	}
	if h.a.term.active != t2 || h.a.term.focus != t2.active {
		t.Fatalf("click selected tab %p focus %p", h.a.term.active, h.a.term.focus)
	}
	_ = t1
}

func TestMoveAndCloseOtherTermTabs(t *testing.T) {
	h, t1, t2 := termTabsHarness(t)
	h.a.moveTermTab(t2, 0)
	if h.a.term.tabs[0] != t2 || h.a.term.tabs[1] != t1 {
		t.Fatalf("move made %v", h.a.term.tabs)
	}
	h.a.closeOtherTermTabs(t2)
	if len(h.a.term.tabs) != 1 || h.a.term.tabs[0] != t2 {
		t.Fatalf("close others kept %v", h.a.term.tabs)
	}
	h.a.closeTermTab(t2)
	if len(h.a.term.tabs) != 0 || h.a.term.open {
		t.Fatalf("closing the last tab kept the panel: %+v", h.a.term)
	}
}

func TestRenameTermTab(t *testing.T) {
	h, t1, _ := termTabsHarness(t)
	h.a.startTermRename(t1)
	h.a.term.rename = "  logs  "
	h.a.finishTermRename(true)
	if t1.name != "logs" {
		t.Fatalf("rename kept %q", t1.name)
	}
	h.a.startTermRename(t1)
	h.a.term.rename = "gone"
	h.a.finishTermRename(false)
	if t1.name != "logs" {
		t.Fatalf("escape kept %q", t1.name)
	}
}

func TestZoomTermPaneToggles(t *testing.T) {
	h, t1, _ := termTabsHarness(t)
	h.a.toggleTermZoom()
	if t1.zoom != t1.active {
		t.Fatalf("zoom %p want %p", t1.zoom, t1.active)
	}
	h.a.toggleTermZoom()
	if t1.zoom != nil {
		t.Fatalf("unzoom kept %p", t1.zoom)
	}
}

// A zoomed pane says how many panes it hides, and the chip brings them
// back: the header's button alone was easy to miss.
func TestZoomedPaneShowsHiddenPanesChip(t *testing.T) {
	h := newHarness(t, writeTree(t, map[string]string{"a.txt": "text"}))
	h.open("a.txt")
	a := &termPane{id: 1, title: "zsh"}
	b := &termPane{id: 2, title: "zsh"}
	root := &termNode{id: 1, first: &termNode{id: 1, pane: a}, second: &termNode{id: 2, pane: b}, ratio: 50}
	tab := &termTab{id: 1, root: root, active: b, zoom: b}
	h.a.term = termPanel{open: true, view: panelTerminal, maximized: true, tabs: []*termTab{tab}, active: tab, focus: b}
	h.tt.Frame()
	if !h.tt.HasText(locales.MsgTerminalPanesHidden("1")) {
		t.Fatalf("hidden panes chip missing: %v", h.tt.Texts())
	}
	if err := h.tt.Click(locales.MsgTerminalPanesHidden("1")); err != nil {
		t.Fatal(err)
	}
	if tab.zoom != nil {
		t.Fatalf("chip kept the zoom %p", tab.zoom)
	}
}
