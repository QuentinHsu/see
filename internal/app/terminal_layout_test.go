package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"see/internal/config"
)

func TestTermNodeSplitAndRemove(t *testing.T) {
	a := &termPane{id: 1, title: "a"}
	b := &termPane{id: 2, title: "b"}
	c := &termPane{id: 3, title: "c"}
	root := &termNode{id: a.id, pane: a}
	if !root.split(a, b, false, 10) {
		t.Fatal("split did not find its anchor")
	}
	if root.pane != nil || root.vertical || root.ratio != 50 || root.first.pane != a || root.second.pane != b {
		t.Fatalf("split: %+v", root)
	}
	if !root.split(b, c, true, 11) {
		t.Fatal("nested split did not find its anchor")
	}
	if got := len((&termTab{root: root}).panes()); got != 3 {
		t.Fatalf("panes: %d", got)
	}
	root, ok := root.remove(a)
	if !ok {
		t.Fatal("remove missed a")
	}
	if root.pane != nil || root.first.pane != b {
		t.Fatalf("removing the first leaf did not promote its split: %+v", root)
	}
	root, _ = root.remove(b)
	if root == nil || root.pane != c {
		t.Fatalf("removing a branch did not promote its sibling: %+v", root)
	}
}

func TestClosePanePromotesSiblingAndKeepsFocus(t *testing.T) {
	h := newHarness(t, "")
	a := &termPane{id: 1, title: "a"}
	b := &termPane{id: 2, title: "b"}
	tab := &termTab{id: 1, root: &termNode{id: 1, first: &termNode{id: 1, pane: a}, second: &termNode{id: 2, pane: b}, ratio: 50}, active: a}
	h.a.term = termPanel{open: true, view: panelTerminal, tabs: []*termTab{tab}, active: tab, focus: a}
	h.a.closePane(a)
	if tab.root.pane != b || tab.active != b || h.a.term.focus != b {
		t.Fatalf("close kept root %+v active %p focus %p", tab.root, tab.active, h.a.term.focus)
	}
	if len(h.a.term.tabs) != 1 || !h.a.term.open {
		t.Fatalf("closing a pane closed the tab: %+v", h.a.term)
	}
	h.a.closePane(b)
	if len(h.a.term.tabs) != 0 || h.a.term.open {
		t.Fatalf("closing the last pane kept the panel: %+v", h.a.term)
	}
}

func TestFocusTermPaneByDirection(t *testing.T) {
	h := newHarness(t, "")
	left := &termPane{id: 1, bounds: ui.Rect{X: 0, Y: 0, W: 100, H: 100}}
	right := &termPane{id: 2, bounds: ui.Rect{X: 100, Y: 0, W: 100, H: 100}}
	below := &termPane{id: 3, bounds: ui.Rect{X: 0, Y: 100, W: 100, H: 100}}
	tab := &termTab{id: 1, root: &termNode{id: 1, pane: left}, active: left}
	h.a.term = termPanel{open: true, view: panelTerminal, tabs: []*termTab{tab}, active: tab, focus: left}
	tab.root = &termNode{id: 1, first: &termNode{id: 1, pane: left}, second: &termNode{id: 2, first: &termNode{id: 2, pane: right}, second: &termNode{id: 3, pane: below}, vertical: true}, ratio: 50}
	h.a.focusTermPane(termFocusRight)
	if tab.active != right || h.a.term.focus != right {
		t.Fatalf("right moved to %p", tab.active)
	}
	tab.active, h.a.term.focus = left, left
	h.a.focusTermPane(termFocusDown)
	if tab.active != below || h.a.term.focus != below {
		t.Fatalf("down moved to %p", tab.active)
	}
}

func TestTermProgramFromTitle(t *testing.T) {
	cases := []struct {
		title, shell, name string
		shellProgram       bool
	}{
		{"quentin@host: ~/code", "zsh", "zsh", true},
		{"~/code/see", "zsh", "zsh", true},
		{"zsh", "zsh", "zsh", true},
		{"main.go - VIM", "zsh", "Vim", false},
		{"lazygit", "zsh", "Git Changes", false},
		{"node", "zsh", "Node", false},
		{"a title from some program", "zsh", "a title from some program", false},
	}
	for _, tc := range cases {
		prog := termProgramFor(tc.title, tc.shell)
		if prog.Name != tc.name || prog.Shell != tc.shellProgram {
			t.Errorf("%q: got %q shell=%v, want %q shell=%v", tc.title, prog.Name, prog.Shell, tc.name, tc.shellProgram)
		}
	}
}

func TestTermSessionKeepsNameTreeAndZoom(t *testing.T) {
	h := newHarness(t, "")
	a := &termPane{id: 1, title: "zsh"}
	b := &termPane{id: 2, title: "vim"}
	root := &termNode{id: 1, first: &termNode{id: 1, pane: a}, second: &termNode{id: 2, pane: b}, vertical: true, ratio: 35}
	tab := &termTab{id: 1, name: "dev", root: root, active: b, zoom: b}
	h.a.term = termPanel{open: true, view: panelTerminal, tabs: []*termTab{tab}, active: tab}
	tabs, active, open := h.a.termSession()
	if !open || active != 0 || len(tabs) != 1 {
		t.Fatalf("session: open=%v active=%d tabs=%+v", open, active, tabs)
	}
	saved := tabs[0]
	if saved.Name != "dev" || saved.Active != 1 || saved.Zoom != 2 {
		t.Fatalf("saved: %+v", saved)
	}
	if saved.Root == nil || !saved.Root.Vertical || saved.Root.Ratio != 35 || !saved.Root.Second.Leaf || saved.Root.Second.Pane != 1 {
		t.Fatalf("saved root: %+v", saved.Root)
	}
}

func TestTermSessionRestoresSplitTree(t *testing.T) {
	panes := []*termPane{{id: 1}, {id: 2}, {id: 3}}
	saved := &config.TermLayout{
		Vertical: true,
		Ratio:    40,
		First:    &config.TermLayout{Leaf: true, Pane: 0},
		Second: &config.TermLayout{
			Ratio:  60,
			First:  &config.TermLayout{Leaf: true, Pane: 1},
			Second: &config.TermLayout{Leaf: true, Pane: 2},
		},
	}
	root := restoreTermLayout(saved, panes)
	if root == nil || root.pane != nil || !root.vertical || root.ratio != 40 {
		t.Fatalf("root: %+v", root)
	}
	if root.second == nil || root.second.vertical || root.second.ratio != 60 {
		t.Fatalf("nested split: %+v", root.second)
	}
	if again := restoreTermLayout(saveTermLayout(root, map[*termPane]int{panes[0]: 0, panes[1]: 1, panes[2]: 2}), panes); again == nil || len((&termTab{root: again}).panes()) != 3 {
		t.Fatalf("round trip lost panes: %+v", again)
	}
}
