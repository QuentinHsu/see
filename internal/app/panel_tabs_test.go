package app

import (
	"testing"

	"see/locales"
)

// The panel's views are tabs across its top, and each one selects its view.
func TestPanelViewTabsSelectViews(t *testing.T) {
	h := newHarness(t, writeTree(t, map[string]string{"a.txt": "text"}))
	h.open("a.txt")
	h.a.term = termPanel{open: true, view: panelOutput}
	h.tt.Frame()
	for _, label := range []string{locales.MsgOutput(), taskMessage("tasksTitle"), locales.MsgReferences(), h.a.hierarchyText("")} {
		if !h.tt.HasText(label) {
			t.Fatalf("panel tab %q missing: %v", label, h.tt.Texts())
		}
	}
	if err := h.tt.Click(locales.MsgReferences()); err != nil {
		t.Fatal(err)
	}
	if h.a.term.view != panelReferences {
		t.Fatalf("clicking References selected view %d", h.a.term.view)
	}
	h.tt.Frame()
	if err := h.tt.Click(taskMessage("tasksTitle")); err != nil {
		t.Fatal(err)
	}
	if h.a.term.view != panelTasks {
		t.Fatalf("clicking Tasks selected view %d", h.a.term.view)
	}
}
