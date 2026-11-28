package app

import (
	"os"
	"path/filepath"
	"regexp"
	"see/internal/config"
	"see/locales"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// The bottom panel: a tab for each terminal, its panes split right and
// down as cards with their program, title and directory, as Rex draws
// them. The terminals run with MyGo's terminal plugin, Ghostty's
// emulator.

type panelView uint8

const (
	panelTerminal panelView = iota
	panelOutput
	panelTasks
	panelReferences
	panelCalls
)

// termPanel is the bottom panel's state.
type termPanel struct {
	open      bool
	view      panelView
	maximized bool
	tabs      []*termTab
	active    *termTab
	pct       float32 // the editors' share of the height, as a percentage
	nextID    int
	loading   bool  // the emulator's library is loading
	loadErr   error // it could not
	focus     *termPane
	focusReq  *termPane // a terminal to focus once its view is built
	focused   bool      // a terminal had the keyboard in the last frame
	wasMax    bool      // the layout the last frame drew
	renaming  *termTab
	rename    string
	renameOff bool // whether the field had the keyboard
}

// termTab is a tab of the panel: one terminal, or a tree of them split
// right and down, as Rex arranges panes.
type termTab struct {
	id     int
	name   string // the user's name for it; empty follows the focused pane
	root   *termNode
	active *termPane
	zoom   *termPane // the pane the tab shows alone, or nil
}

// termNode is a pane, or two nodes split across: vertical when the first
// is above the second. ratio is the first's share, as a percentage.
type termNode struct {
	id            int
	pane          *termPane
	first, second *termNode
	vertical      bool
	ratio         float32
}

// termPane is one terminal.
type termPane struct {
	id        int
	term      *terminal.Terminal
	title     string
	attention bool
	lastText  string
	lastSeen  time.Time // the last time its program printed
	polled    time.Time
	bounds    ui.Rect // where the last frame drew it, to focus by direction
}

// newTerm makes a terminal running the user's shell in dir, or in the
// workspace when dir is empty or gone. Its callbacks come from the
// terminal's goroutines: they go through post, and are dropped once the
// pane or the workspace is gone.
func (a *App) newTerm(p *termPane, dir string) error {
	gen := a.gen
	var t *terminal.Terminal
	t, err := terminal.New(terminal.Options{
		Dir: a.termDir(dir),
		Command: func() []string {
			if a.cfg.TermShell == "" {
				return nil
			}
			return []string{a.cfg.TermShell}
		}(),
		Font: a.terminalFont(),
		Cursor: func() terminal.CursorStyle {
			switch a.cfg.TermCursor {
			case "bar":
				return terminal.CursorBar
			case "underline":
				return terminal.CursorUnderline
			}
			return terminal.CursorBlock
		}(),
		NoBlink:     a.cfg.TermCursorNoBlink,
		OptionAsAlt: a.cfg.TermOptionAsAlt,
		CopyOnSelect: map[bool]terminal.CopyOnSelect{
			true: terminal.CopyOnSelectClipboard,
		}[a.cfg.TermCopyOnSelect],
		RightClick: func() terminal.RightClickAction {
			switch a.cfg.TermRightClick {
			case "paste":
				return terminal.RightClickPaste
			case "copyOrPaste":
				return terminal.RightClickCopyOrPaste
			}
			return terminal.RightClickContextMenu
		}(),
		ClipboardRead: func() terminal.ClipboardAccess {
			switch a.cfg.TermClipboard {
			case "allow":
				return terminal.ClipboardAllow
			case "deny":
				return terminal.ClipboardDeny
			}
			return terminal.ClipboardAsk
		}(),
		Scrollback: func() int {
			v := a.cfg.TermScrollback
			if v == 0 {
				v = 4
			}
			if v < 0 {
				return -1
			}
			return v << 20
		}(),
		Env:             a.cfg.TermEnv,
		MinimumContrast: float32(a.cfg.TermMinContrast),
		OnTitle: func(title string) {
			a.post.Post(func() {
				if a.gen == gen {
					p.title = title
				}
			})
		},
		OnExit: func(int) {
			a.post.Post(func() {
				if a.gen == gen {
					a.closePane(p) // as VS Code closes a terminal whose shell ended
				}
			})
		},
		OnBell: func() {
			a.post.Post(func() {
				if a.gen == gen {
					p.attention = true
					a.notifyTerminal(p.title, "")
				}
			})
		},
		OnNotify: func(title, body string) {
			a.post.Post(func() {
				if a.gen == gen {
					p.attention = true
					a.notifyTerminal(title, body)
				}
			})
		},
		OpenLink: func(link string) bool { return a.openTermLink(p, link) },
	})
	if err != nil {
		return err
	}
	p.term = t
	return nil
}

// withTerminal runs fn once the emulator's library is loaded, which may
// download it the first time under go run and go test, off the UI
// thread; packaged apps carry it.
func (a *App) withTerminal(fn func()) {
	tp := &a.term
	if tp.loadErr != nil || tp.loading {
		return
	}
	if termLoaded {
		fn()
		return
	}
	tp.loading = true
	gen := a.gen
	go func() {
		err := terminal.Load()
		a.post.Post(func() {
			tp.loading = false
			if err != nil {
				tp.loadErr = err
				return
			}
			termLoaded = true
			if a.gen == gen {
				fn()
			}
		})
	}()
}

var termLoaded bool

// toggleTerminal shows or hides the panel, with a terminal in it.
func (a *App) toggleTerminal() {
	tp := &a.term
	if tp.open {
		tp.open, tp.maximized = false, false
		tp.focused = false
		tp.focus = nil
		if a.focus != nil {
			a.focusEditor = a.focus.eds[a.focus.active]
		}
		return
	}
	tp.open = true
	tp.view = panelTerminal
	a.focusEditor = nil
	if len(tp.tabs) == 0 {
		a.newTerminalTab()
	} else if tp.active != nil {
		tp.focus = tp.active.active
		if tp.active.zoom != nil {
			tp.focus = tp.active.zoom
		}
		tp.focusReq = tp.focus
	}
}

// newTerminalTab opens a tab with a new terminal at the workspace root.
func (a *App) newTerminalTab() { a.newTerminalTabAt(a.root) }

// newTerminalTabAt opens a tab with a new terminal in dir.
func (a *App) newTerminalTabAt(dir string) {
	if dir == "" {
		return
	}
	a.term.open = true
	a.term.view = panelTerminal
	a.focusEditor = nil
	a.withTerminal(func() {
		tp := &a.term
		p, ok := a.spawnTermPane(dir, a.shellTitle())
		if !ok {
			return
		}
		tp.nextID++
		tab := &termTab{id: tp.nextID, root: &termNode{id: p.id, pane: p}, active: p}
		tp.tabs = append(tp.tabs, tab)
		tp.active, tp.focus, tp.focusReq = tab, p, p
	})
}

// spawnTermPane starts a terminal in dir, with termPane's title until
// its shell names it, and reports whether it started.
func (a *App) spawnTermPane(dir, title string) (*termPane, bool) {
	tp := &a.term
	tp.nextID++
	p := &termPane{id: tp.nextID, title: title}
	if err := a.newTerm(p, dir); err != nil {
		a.fail(locales.MsgCouldNotStartATerminal() + err.Error())
		return nil, false
	}
	return p, true
}

// splitTerminal splits the active pane across: vertical puts the new
// terminal below it, as Rex's split down; otherwise beside it.
func (a *App) splitTerminal(vertical bool) {
	tp := &a.term
	tab := tp.active
	if tab == nil {
		a.newTerminalTab()
		return
	}
	anchor := tab.active
	if tab.zoom != nil {
		anchor, tab.zoom = tab.zoom, nil
	}
	if anchor == nil {
		return
	}
	a.withTerminal(func() {
		tp := &a.term
		p, ok := a.spawnTermPane(a.paneDir(anchor), a.shellTitle())
		if !ok {
			return
		}
		tp.nextID++
		if !tab.root.split(anchor, p, vertical, tp.nextID) {
			if p.term != nil {
				p.term.Close()
			}
			return
		}
		tab.active, tp.focus, tp.focusReq = p, p, p
		tp.open, tp.view = true, panelTerminal
	})
}

// closePane closes a terminal: its split goes with it, its tab with its
// last terminal, and the panel with its last tab.
func (a *App) closePane(p *termPane) {
	tp := &a.term
	for ti, tab := range tp.tabs {
		order := tab.panes()
		i := slices.Index(order, p)
		if i < 0 {
			continue
		}
		if p.term != nil {
			p.term.Close()
		}
		if tab.zoom == p {
			tab.zoom = nil
		}
		if tab.root != nil {
			tab.root, _ = tab.root.remove(p)
		}
		left := tab.panes()
		switch {
		case len(left) == 0:
			tp.tabs = slices.Delete(tp.tabs, ti, ti+1)
			if tp.active == tab {
				tp.active = nil
				if n := len(tp.tabs); n > 0 {
					tp.active = tp.tabs[min(ti, n-1)]
				}
			}
		case tab.active == p:
			tab.active = left[min(i, len(left)-1)]
		}
		if tp.focus == p {
			tp.focus = nil
			if tp.active != nil {
				tp.focus = tp.active.active
				if tp.active.zoom != nil {
					tp.focus = tp.active.zoom
				}
				tp.focusReq = tp.focus
			} else if a.focus != nil {
				a.focusEditor = a.focus.eds[a.focus.active]
			}
		}
		if len(tp.tabs) == 0 {
			tp.open, tp.maximized = false, false
			tp.renaming = nil
		}
		return
	}
}

// termTabIndex is where tab sits in the panel, or -1.
func (a *App) termTabIndex(tab *termTab) int {
	return slices.Index(a.term.tabs, tab)
}

// selectTermTab shows tab, and its focused pane.
func (a *App) selectTermTab(tab *termTab) {
	if tab == nil {
		return
	}
	tp := &a.term
	tp.active, tp.view = tab, panelTerminal
	if tab.zoom != nil {
		tp.focus = tab.zoom
	} else {
		tp.focus = tab.active
	}
	tp.focusReq = tp.focus
}

// cycleTermTab moves through the panel's tabs, dir steps around.
func (a *App) cycleTermTab(dir int) {
	tp := &a.term
	n := len(tp.tabs)
	if n < 2 {
		return
	}
	i := max(a.termTabIndex(tp.active), 0)
	a.selectTermTab(tp.tabs[(i+dir+n)%n])
}

// activateTermTabNumber shows the n-th terminal tab, counting from one.
func (a *App) activateTermTabNumber(n int) {
	tp := &a.term
	if n < 1 || n > len(tp.tabs) {
		return
	}
	a.selectTermTab(tp.tabs[n-1])
}

// moveTermTab moves tab to index to, as dragging it across the strip.
func (a *App) moveTermTab(tab *termTab, to int) {
	tp := &a.term
	from := a.termTabIndex(tab)
	if from < 0 {
		return
	}
	tp.tabs = slices.Delete(tp.tabs, from, from+1)
	tp.tabs = slices.Insert(tp.tabs, min(to, len(tp.tabs)), tab)
}

// closeTermTab closes a tab and every terminal in it.
func (a *App) closeTermTab(tab *termTab) {
	for _, p := range tab.panes() {
		if p.term != nil {
			p.term.Close()
		}
	}
	tp := &a.term
	i := a.termTabIndex(tab)
	if i < 0 {
		return
	}
	tp.tabs = slices.Delete(tp.tabs, i, i+1)
	if tp.renaming == tab {
		tp.renaming = nil
	}
	if tp.active == tab {
		tp.active = nil
		if n := len(tp.tabs); n > 0 {
			tp.active = tp.tabs[min(i, n-1)]
			tp.focus = tp.active.active
			if tp.active.zoom != nil {
				tp.focus = tp.active.zoom
			}
		}
	}
	if len(tp.tabs) == 0 {
		tp.open, tp.maximized = false, false
		tp.focus = nil
		if a.focus != nil {
			a.focusEditor = a.focus.eds[a.focus.active]
		}
	}
}

// closeOtherTermTabs closes every tab but tab.
func (a *App) closeOtherTermTabs(tab *termTab) {
	for _, other := range slices.Clone(a.term.tabs) {
		if other != tab {
			a.closeTermTab(other)
		}
	}
}

// toggleTermZoom shows the focused pane alone, or the tab again.
func (a *App) toggleTermZoom() {
	tab := a.term.active
	if tab == nil {
		return
	}
	if tab.zoom != nil {
		tab.zoom = nil
	} else {
		tab.zoom = tab.active
	}
	if tab.zoom != nil {
		a.term.focusReq = tab.zoom
	} else {
		a.term.focusReq = tab.active
	}
}

// startTermRename edits a tab's name in place.
func (a *App) startTermRename(tab *termTab) {
	tp := &a.term
	tp.renaming, tp.rename = tab, tab.name
	tp.renameOff = false
}

// finishTermRename keeps or drops the name being edited.
func (a *App) finishTermRename(keep bool) {
	tp := &a.term
	if keep && tp.renaming != nil {
		name := strings.TrimSpace(tp.rename)
		if tp.renaming.name != name {
			tp.renaming.name = name
			a.saveConfigSoon()
		}
	}
	tp.renaming = nil
}

// termSession is what of the panel a session keeps: each tab's shells,
// their directories, titles and splits, which tab was active, and
// whether the panel showed terminals at all.
func (a *App) termSession() (tabs []config.TermTab, active int, open bool) {
	tp := &a.term
	if tp.view != panelTerminal {
		return nil, 0, false
	}
	for _, tab := range tp.tabs {
		panes := tab.panes()
		if len(panes) == 0 {
			continue
		}
		index := make(map[*termPane]int, len(panes))
		saved := config.TermTab{Name: tab.name}
		for i, p := range panes {
			index[p] = i
			sp := config.TermPane{Title: p.title}
			if p.term != nil {
				sp.Dir = p.term.Dir()
			}
			saved.Panes = append(saved.Panes, sp)
		}
		saved.Root = saveTermLayout(tab.root, index)
		if i, ok := index[tab.active]; ok {
			saved.Active = i
		}
		if i, ok := index[tab.zoom]; ok {
			saved.Zoom = i + 1
		}
		if tp.active == tab {
			active = len(tabs)
		}
		tabs = append(tabs, saved)
	}
	if len(tabs) == 0 {
		return nil, 0, false
	}
	return tabs, active, tp.open
}

// saveTermLayout writes a split tree, its panes as indices in the tab's
// saved panes.
func saveTermLayout(n *termNode, index map[*termPane]int) *config.TermLayout {
	if n == nil {
		return nil
	}
	if n.pane != nil {
		i, ok := index[n.pane]
		if !ok {
			return nil
		}
		return &config.TermLayout{Leaf: true, Pane: i}
	}
	first := saveTermLayout(n.first, index)
	second := saveTermLayout(n.second, index)
	switch {
	case first == nil:
		return second
	case second == nil:
		return first
	}
	return &config.TermLayout{Vertical: n.vertical, Ratio: n.ratio, First: first, Second: second}
}

// openRestoredTerminals recreates the panel's tabs of the last session:
// a fresh shell at each one's directory, with its title, and its splits.
// The first failure ends the restore; terminals opened before it stay.
func (a *App) openRestoredTerminals(s config.Workspace) {
	if len(s.Terminals) == 0 || a.root == "" {
		return
	}
	tp := &a.term
	tp.open, tp.view = s.PanelOpen, panelTerminal
	tp.focus = nil
	a.withTerminal(func() {
		if a.root == "" || len(tp.tabs) > 0 {
			return
		}
		for i, saved := range s.Terminals {
			tp.nextID++
			tab := &termTab{id: tp.nextID, name: saved.Name}
			panes := make([]*termPane, len(saved.Panes))
			failed := false
			for j, sp := range saved.Panes {
				tp.nextID++
				p := &termPane{id: tp.nextID, title: sp.Title}
				if err := a.newTerm(p, sp.Dir); err != nil {
					a.logf("session: restoring terminal: %v", err)
					failed = true
					break
				}
				panes[j] = p
			}
			if failed {
				for _, p := range panes {
					if p != nil && p.term != nil {
						p.term.Close()
					}
				}
				tp.open = s.PanelOpen && len(tp.tabs) > 0
				return
			}
			if len(panes) == 0 {
				continue
			}
			tab.root = restoreTermLayout(saved.Root, panes)
			if tab.root == nil {
				tab.root = legacyTermTree(panes)
			}
			order := tab.panes()
			tab.active = order[min(max(saved.Active, 0), len(order)-1)]
			if saved.Zoom > 0 && saved.Zoom <= len(order) {
				tab.zoom = order[saved.Zoom-1]
			}
			tp.tabs = append(tp.tabs, tab)
			if i == s.ActiveTerminal {
				tp.active = tab
			}
		}
		if tp.active == nil && len(tp.tabs) > 0 {
			tp.active = tp.tabs[0]
		}
		if tp.active != nil {
			tp.focus = tp.active.active
			if tp.active.zoom != nil {
				tp.focus = tp.active.zoom
			}
		}
		if len(tp.tabs) == 0 {
			tp.open = false
		}
	})
}

// restoreTermLayout rebuilds a saved split tree over its panes.
func restoreTermLayout(l *config.TermLayout, panes []*termPane) *termNode {
	if l == nil {
		return nil
	}
	if l.Leaf || (l.First == nil && l.Second == nil) {
		if l.Pane < 0 || l.Pane >= len(panes) {
			return nil
		}
		return &termNode{id: panes[l.Pane].id, pane: panes[l.Pane]}
	}
	first := restoreTermLayout(l.First, panes)
	second := restoreTermLayout(l.Second, panes)
	switch {
	case first == nil:
		return second
	case second == nil:
		return first
	}
	ratio := l.Ratio
	if ratio <= 0 || ratio >= 100 {
		ratio = 50
	}
	return &termNode{id: first.id, first: first, second: second, vertical: l.Vertical, ratio: ratio}
}

// legacyTermTree lays panes saved before splits were a tree side by side.
func legacyTermTree(panes []*termPane) *termNode {
	last := len(panes) - 1
	n := &termNode{id: panes[last].id, pane: panes[last]}
	for i := last - 1; i >= 0; i-- {
		n = &termNode{
			id:     panes[i].id,
			first:  &termNode{id: panes[i].id, pane: panes[i]},
			second: n,
			ratio:  100 / float32(last-i+1),
		}
	}
	return n
}

// closeTerminals ends every terminal, as the workspace closes.
func (a *App) closeTerminals() {
	for _, tab := range a.term.tabs {
		for _, p := range tab.panes() {
			if p.term != nil {
				p.term.Close()
			}
		}
	}
	pct := a.term.pct
	a.term = termPanel{pct: pct, loadErr: a.term.loadErr}
}

// termDir is where a restored or new terminal starts: dir while it
// still is a directory of the workspace, the workspace's root else.
func (a *App) termDir(dir string) string {
	if dir == "" || a.root == "" || !a.withinWorkspace(dir) {
		return a.root
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return a.root
	}
	return dir
}

// paneDir is the working directory the pane's shell reported (OSC 7),
// or the workspace's root when the shell reported none, as one not
// running yet. A split starts where the split pane runs.
func (a *App) paneDir(p *termPane) string {
	if p != nil && p.term != nil && p.term.Dir() != "" {
		return p.term.Dir()
	}
	return a.root
}

// shellTitle is a new pane's title before its shell's first OSC title:
// the name of the shell that will run.
func (a *App) shellTitle() string {
	if a.cfg.TermShell != "" {
		return filepath.Base(a.cfg.TermShell)
	}
	return shellName()
}

func shellName() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		return filepath.Base(sh)
	}
	return "shell"
}

// termLink matches a path printed with a line, and a column: main.go:12:3.
var termLink = regexp.MustCompile(`^(.+?)(?::(\d+))?(?::(\d+))?$`)

// openTermLink opens a file path a terminal printed, which Cmd+click
// opens, in the editor, at its line and column; other links open with
// the system. It runs on the UI thread.
func (a *App) openTermLink(p *termPane, link string) bool {
	if strings.Contains(link, "://") {
		return false
	}
	m := termLink.FindStringSubmatch(link)
	if m == nil {
		return false
	}
	path := m[1]
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	if !filepath.IsAbs(path) {
		dir := a.root
		if p.term != nil && p.term.Dir() != "" {
			dir = p.term.Dir()
		}
		path = filepath.Join(dir, path)
	}
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return false
	}
	var pos *editor.Pos
	if m[2] != "" {
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		pos = &editor.Pos{Line: max(line-1, 0), Col: max(col-1, 0)}
	}
	a.cancelNav()
	a.openFile(path, pos)
	if st := a.activeEditor(); st != nil {
		a.focusEditor = st
	}
	return true
}

// ----- View -----

// editorsAndPanel lays out the editors over the bottom panel, which the
// divider between them resizes, as Xcode's debug area.
func (a *App) editorsAndPanel(c *ui.Context, editors func()) {
	tp := &a.term
	if tp.pct <= 0 {
		tp.pct = float32(a.cfg.EditorPanelPct())
	}
	// A layout change, as maximizing the panel, rebuilds the panes and
	// takes the keyboard from the terminal: ask for it back.
	if tp.wasMax != tp.maximized {
		tp.wasMax = tp.maximized
		if tp.view == panelTerminal && tp.active != nil {
			tp.focusReq = tp.active.active
			if tp.active.zoom != nil {
				tp.focusReq = tp.active.zoom
			}
		}
	}
	switch {
	case !tp.open:
		editors()
	case tp.maximized:
		a.terminalPanel(c)
	default:
		if ui.SplitVerticalPercent(c, &tp.pct, editors, func() { a.terminalPanel(c) }).Grow(1).FillWidth().Changed() {
			a.cfg.PanelPct = float64(tp.pct)
			a.saveConfigSoon()
		}
	}
}

func (a *App) terminalPanel(c *ui.Context) {
	tp := &a.term
	panel := ui.Column(c).Fill().Background(colors(c).Editor).BorderWidth(1, 0, 0, 0).BorderColor(colors(c).Hairline)
	panel.Children(func() {
		a.termViewBar(c)
		if tp.view == panelTerminal {
			a.termTabsBar(c)
		}
		ui.Column(c).Grow(1).FillWidth().Children(func() {
			switch {
			case tp.view == panelTasks:
				a.taskView(c)
			case tp.view == panelReferences:
				a.referencesView(c)
			case tp.view == panelOutput:
				a.logView(c)
			case tp.view == panelCalls:
				a.callsPanel(c)
			case tp.loadErr != nil:
				a.message(c, locales.TerminalUnavailable(), tp.loadErr.Error(), "", nil)
			case tp.loading:
				ui.Row(c).Grow(1).FillWidth().Center().Gap(8).Children(func() {
					ui.Spinner(c)
					ui.Text(c, locales.MsgLoadingTheTerminal()).TextColor(colors(c).Muted)
				})
			default:
				a.termPanes(c)
			}
		})
	})
	tp.focused = tp.view == panelTerminal && panel.FocusWithin()
}

var (
	icPlus     = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>`))
	icTrash    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 7h16M9 7V4.5h6V7M6.5 7l1 13h9l1-13M10 11v6M14 11v6"/></svg>`))
	icMaximize = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 15l6-6 6 6"/></svg>`))
	icRestore  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6"/></svg>`))
	icPrompt   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 7l5 5-5 5M12 18h7"/></svg>`))
)

func (a *App) terminalFont() terminal.Font {
	f := terminal.Font{Family: a.cfg.TermFontFamily, Size: float32(a.cfg.TerminalFontSize()), Weight: a.cfg.TermFontWeight, Thicken: a.cfg.TermThicken}
	if a.cfg.TermFontFallback != "" {
		if f.Family == "" {
			f.Family = "monospace"
		}
		f.Family += ", " + a.cfg.TermFontFallback
	}
	if a.cfg.TermLineHeight >= -20 && a.cfg.TermLineHeight <= 100 {
		f.LineHeight = float32(1 + a.cfg.TermLineHeight/100)
	}
	if a.cfg.TermNoLigatures {
		f.Features = append([]string{"-liga", "-calt"}, f.Features...)
	}
	if len(a.cfg.TermFontFeatures) > 0 {
		f.Features = append(slices.Clone(a.cfg.TermFontFeatures), f.Features...)
	}
	return f
}

// applyTerminalPrefs gives running terminals the Terminal pane's settings.
func (a *App) applyTerminalPrefs() {
	cos := terminal.CopyOnSelect(0)
	if a.cfg.TermCopyOnSelect {
		cos = terminal.CopyOnSelectClipboard
	}
	for _, tab := range a.term.tabs {
		for _, p := range tab.panes() {
			if p.term != nil {
				p.term.SetFont(a.terminalFont())
				p.term.SetCopyOnSelect(cos)
				p.term.SetMinimumContrast(float32(a.cfg.TermMinContrast))
			}
		}
	}
}
