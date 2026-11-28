package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"see/locales"
)

// The panel's terminals, as Rex draws them: panes as cards in a split
// tree, each with its program, title and directory over a header of its
// own buttons. The panel's views, Terminal among them, are tabs in the
// bar above; the terminals are tabs in the bar below it. A pane's header
// names its program from the title the terminal reports, and its dot
// follows the screen's changes: the plugin keeps the foreground process
// to itself.

const (
	termBarH    = 32 // the terminal tabs' bar
	termTabH    = 26 // a terminal tab
	termHeaderH = 30 // a pane's header
	termGap     = 6  // around and between panes
)

// termViewBar is the panel's views, across its top, and its actions.
func (a *App) termViewBar(c *ui.Context) {
	tp := &a.term
	bar := ui.Row(c).FillWidth().Height(30).Shrink(0).Background(colors(c).TabBar).
		AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(colors(c).Hairline)
	bar.Children(func() {
		ui.ScrollHorizontal(c.Key("panel-tabs")).Grow(1).MinWidth(0).FillHeight().Scrollbars(false).Children(func() {
			ui.Row(c).FillHeight().Padding(0, 4).Gap(2).AlignItems(ui.Center).Children(func() {
				a.termViewTab(c, panelTerminal, "terminal", locales.Terminal(), icPrompt)
				a.termViewTab(c, panelOutput, "output", locales.MsgOutput(), icLines)
				a.termViewTab(c, panelTasks, "tasks", taskMessage("tasksTitle"), nil)
				a.termViewTab(c, panelReferences, "references", locales.MsgReferences(), icPin)
				a.termViewTab(c, panelCalls, "calls", a.hierarchyText(""), icRelated)
			})
		})
		ui.Row(c).FillHeight().Shrink(0).Padding(0, 4).Gap(1).AlignItems(ui.Center).Children(func() {
			icon := func(icon *ui.SVG, label string) bool {
				return termIconButton(c, icon, label, 26, 14).Clicked()
			}
			maxIcon, maxLabel := icMaximize, locales.MsgMaximizeThePanel()
			if tp.maximized {
				maxIcon, maxLabel = icRestore, locales.MsgRestoreThePanel()
			}
			if icon(maxIcon, maxLabel) {
				tp.maximized = !tp.maximized
				if tp.view == panelTerminal && tp.active != nil {
					tp.focusReq = tp.active.active
					if tp.active.zoom != nil {
						tp.focusReq = tp.active.zoom
					}
				}
			}
			if icon(icClose, locales.MsgHideThePanel()) {
				tp.open, tp.maximized = false, false
			}
		})
	})
}

// termViewTab is one of the panel's views, selectable across its top.
func (a *App) termViewTab(c *ui.Context, view panelView, key, label string, icon *ui.SVG) {
	tp := &a.term
	active := tp.view == view
	tab := ui.Row(c.Key("panel-"+key)).Height(24).Padding(0, 9).Gap(6).Radius(5).Shrink(0).
		AlignItems(ui.Center).Label(label)
	switch {
	case active:
		tab.Background(colors(c).TabActive)
	case tab.Hovered():
		tab.Background(colors(c).TabHover)
	}
	tab.OnClick(func() {
		tp.view = view
		if view == panelTerminal {
			a.selectTermTab(tp.active)
		} else {
			tp.focus = nil
		}
	})
	tab.Children(func() {
		color := colors(c).Muted
		if active {
			color = colors(c).Text
		}
		if icon != nil {
			symbolIcon(c, icon, 14, color)
		}
		ui.Text(c, label).FontSize(12).SingleLine().Ellipsis("…").TextColor(color)
	})
}

// termPanes draws the active tab's terminals: the zoomed one alone, or
// its tree of splits.
func (a *App) termPanes(c *ui.Context) {
	tp := &a.term
	tab := tp.active
	if tab == nil || tab.root == nil {
		ui.Box(c).Grow(1).FillWidth().Center().Children(func() {
			if ui.Button(c, commandLabel("terminal.new")).Clicked() {
				a.newTerminalTab()
			}
		})
		return
	}
	ui.Box(c).Grow(1).FillWidth().Padding(termGap).Background(colors(c).Editor).Children(func() {
		if tab.zoom != nil {
			a.termPaneCard(c, tab, tab.zoom)
			return
		}
		a.termNodeView(c, tab, tab.root)
	})
}

// termNodeView lays out a node of a tab's split tree. Its divider is a
// strip of the panel's background, with no line of its own: the pointer
// turns into a resize cursor over it, and a double click puts the split
// back at half.
func (a *App) termNodeView(c *ui.Context, tab *termTab, n *termNode) {
	if n == nil {
		return
	}
	if n.pane != nil {
		a.termPaneCard(c, tab, n.pane)
		return
	}
	var box ui.Element
	if n.vertical {
		box = ui.Column(c)
	} else {
		box = ui.Row(c)
	}
	box.Grow(1).FillWidth().FillHeight().AlignItems(ui.Stretch)
	bounds := box.Bounds()
	box.Children(func() {
		share := min(max(n.ratio, 10), 90) / 100
		first := ui.Box(c).Grow(share).Basis(0).MinWidth(0).MinHeight(0)
		first.Children(func() { a.termNodeView(c, tab, n.first) })
		a.termDivider(c, n, bounds)
		second := ui.Box(c).Grow(1 - share).Basis(0).MinWidth(0).MinHeight(0)
		second.Children(func() { a.termNodeView(c, tab, n.second) })
	})
}

// termDividerHit is the width of the room the pointer drags a divider in.
const termDividerHit = 8

// termDivider shares the room of a split: the pointer resizes it, and
// nothing is drawn, so the panes meet over the panel's own background.
func (a *App) termDivider(c *ui.Context, n *termNode, bounds ui.Rect) {
	d := ui.Box(c).Shrink(0).Role(ui.RoleSplitter).Label(locales.MsgResizePanes())
	if n.vertical {
		d.Height(termDividerHit).FillWidth().Cursor(ui.CursorResizeRow)
	} else {
		d.Width(termDividerHit).FillHeight().Cursor(ui.CursorResizeColumn)
	}
	if dx, dy, ok := d.Dragged(); ok {
		total, move := bounds.W-termDividerHit, dx
		if n.vertical {
			total, move = bounds.H-termDividerHit, dy
		}
		if total > 0 {
			n.ratio = min(max(n.ratio+move/total*100, 10), 90)
		}
	}
	if d.DoubleClicked() {
		n.ratio = 50
	}
}

// termPaneCard draws a pane: its header over its terminal, as a card
// over the panel's background.
func (a *App) termPaneCard(c *ui.Context, tab *termTab, p *termPane) {
	tp := &a.term
	pal := colors(c)
	focused := tab.active == p && p != nil
	card := ui.Column(c.Key(p.id)).Fill().MinWidth(0).MinHeight(0).Radius(9).Clip().
		Background(pal.Sidebar).Border(1, pal.Hairline)
	if focused {
		card.Border(1, c.Theme().Accent.Alpha(0.55))
	}
	card.Children(func() {
		h := ui.Row(c).FillWidth().Height(termHeaderH).Shrink(0).Padding(0, 5, 0, 8).Gap(7).AlignItems(ui.Center)
		h.OnClick(func() {
			tab.active = p
			tp.focus, tp.focusReq = p, p
		})
		if h.DoubleClicked() {
			a.toggleTermZoom()
		}
		h.Children(func() {
			a.termPaneHeader(c, tab, p, focused, card.Hovered())
		})
		body := ui.Box(c).Grow(1).MinHeight(0).FillWidth().Padding(0, 4, 4, 4)
		body.Children(func() {
			if p.term == nil {
				return
			}
			v := terminal.View(c, p.term).Fill().Label(locales.MsgTerminal()+p.title).FocusBind(&tp.focus, p)
			if tp.focusReq == p && tp.view == panelTerminal && tp.active == tab {
				tp.focus, tp.focusReq = p, nil
				v.Focus()
			}
			if v.FocusWithin() {
				tab.active = p
				tp.active = tab
				p.attention = false
			}
		})
	})
	p.bounds = card.Bounds()
}

// termPaneHeader draws a pane's program, title and directory, its
// activity, and its buttons.
func (a *App) termPaneHeader(c *ui.Context, tab *termTab, p *termPane, focused, hovered bool) {
	pal := colors(c)
	a.sampleTermPane(c, p)
	prog := termProgramFor(p.title, a.shellTitle())
	name, detail := a.termPaneLabel(p, prog)
	symbolIcon(c, prog.Glyph, 14, pal.Muted)
	ui.Text(c, name).FontSize(12).FontWeight(600).SingleLine().Ellipsis("…").Shrink(0).MaxWidthPercent(70)
	if detail != "" {
		ui.Text(c, detail).FontSize(11).TextColor(pal.Muted).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
	}
	switch a.termPaneStatus(c, p, prog) {
	case termBusy:
		ui.Box(c).Size(7, 7).Shrink(0).Radius(4).Background(termBusyColor(c))
	case termAttention:
		ui.Box(c).Size(7, 7).Shrink(0).Radius(4).Background(termAttentionColor(c))
	}
	if tab.zoom == p {
		if hidden := len(tab.panes()) - 1; hidden > 0 {
			a.termHiddenPanesChip(c, tab, p, hidden)
		}
	}
	ui.Box(c).Grow(1)
	if !focused && !hovered {
		return
	}
	ui.Row(c).Gap(1).AlignItems(ui.Center).Shrink(0).Children(func() {
		button := func(icon *ui.SVG, label string) bool {
			return termIconButton(c, icon, label, 24, 15).Clicked()
		}
		if button(icSplitH, locales.MsgSplitTerminal()) {
			tab.active, a.term.focus = p, p
			a.splitTerminal(false)
		}
		if button(icSplitV, commandLabel("terminal.splitDown")) {
			tab.active, a.term.focus = p, p
			a.splitTerminal(true)
		}
		zoomIcon, zoomLabel := tpZoomIn, commandLabel("terminal.zoom")
		zoomed := tab.zoom == p
		if zoomed {
			zoomIcon, zoomLabel = tpAllPanes, commandLabel("terminal.unzoom")
		}
		zoomButton := termIconButton(c, zoomIcon, zoomLabel, 24, 15)
		if zoomed {
			zoomButton.Background(c.Theme().Accent.Alpha(0.16))
		}
		if zoomButton.Clicked() {
			tab.active, a.term.focus = p, p
			a.toggleTermZoom()
		}
		if button(icClose, commandLabel("terminal.closePane")) {
			a.closePane(p)
		}
	})
}

// termHiddenPanesChip says how many panes the zoomed tab hides, and
// brings them back: the header's button alone is too easy to miss, and
// a hidden session is one the user forgets.
func (a *App) termHiddenPanesChip(c *ui.Context, tab *termTab, p *termPane, hidden int) {
	accent := c.Theme().Accent
	label := locales.CommandUnzoomTerminalPane() + " (" + locales.MsgTerminalPanesHidden(fmt.Sprint(hidden)) + ")"
	chip := ui.Row(c).Height(18).Padding(0, 7).Gap(4).Radius(9).Shrink(0).AlignItems(ui.Center).
		Background(accent.Alpha(0.14)).Label(label)
	if chip.Hovered() {
		chip.Background(accent.Alpha(0.22))
	}
	chip.OnClick(func() {
		tab.zoom = nil
		tab.active, a.term.focus, a.term.focusReq = p, p, p
	})
	chip.Children(func() {
		symbolIcon(c, tpAllPanes, 11, accent)
		ui.Text(c, locales.MsgTerminalPanesHidden(fmt.Sprint(hidden))).FontSize(11).TextColor(accent)
	})
}

// termIconButton is a borderless button of an icon with a face on hover.
func termIconButton(c *ui.Context, icon *ui.SVG, label string, size, iconSize float32) ui.Element {
	b := ui.ButtonBase(c).Label(label).Size(size, size).Radius(5).Shrink(0).KeepFocus().Center()
	if b.Hovered() || b.Pressed() {
		b.Background(colors(c).Pill)
	}
	b.Children(func() { symbolIcon(c, icon, iconSize, colors(c).Muted) })
	return b
}

// termPaneStatus is what a pane's program is doing.
type termPaneStatus uint8

const (
	termIdle termPaneStatus = iota
	termBusy
	termAttention
)

// sampleTermPane notices a program printing: the screen changed since
// the last look. It reads the screen a few times a second, not every
// frame, as the text is a copy of all of it.
func (a *App) sampleTermPane(c *ui.Context, p *termPane) {
	if p.term == nil {
		return
	}
	now := c.Now()
	if now.Sub(p.polled) < 200*time.Millisecond {
		return
	}
	p.polled = now
	if txt := p.term.Text(); txt != p.lastText {
		p.lastText, p.lastSeen = txt, now
	}
}

// termPaneStatus tells the dot a pane shows: its program printing, or
// done while the user looked elsewhere.
func (a *App) termPaneStatus(c *ui.Context, p *termPane, prog termProgram) termPaneStatus {
	if p.attention {
		return termAttention
	}
	if p.term == nil || prog.Shell {
		return termIdle
	}
	if since := c.Now().Sub(p.lastSeen); since < 1500*time.Millisecond {
		c.After(1500*time.Millisecond - since)
		return termBusy
	}
	return termIdle
}

func termBusyColor(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#4ade80")
	}
	return ui.Hex("#34a853")
}

func termAttentionColor(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#fbbf24")
	}
	return ui.Hex("#f59e0b")
}

// termPaneLabel names a pane: the program, and where it runs. A program
// that set a title of its own is named by it.
func (a *App) termPaneLabel(p *termPane, prog termProgram) (name, detail string) {
	dir := a.termShortDir(a.paneDir(p))
	title := strings.TrimSpace(p.title)
	if prog.Shell || title == "" || titleLooksLikeShell(title, a.shellTitle()) {
		return prog.Name, dir
	}
	return title, dir
}

// termFocusDir is a direction to move the focus of the panel's panes.
type termFocusDir uint8

const (
	termFocusLeft termFocusDir = iota
	termFocusRight
	termFocusUp
	termFocusDown
)

// equalize makes every split of the tab share its room in half, as
// Rex's Control-Command-=.
func (a *App) equalizeTermPanes() {
	if tab := a.term.active; tab != nil {
		tab.root.equalize()
	}
}

// focusTermPane moves the keyboard to the nearest pane in dir, as Rex
// does with the arrows.
func (a *App) focusTermPane(dir termFocusDir) {
	tab := a.term.active
	if tab == nil || tab.zoom != nil || tab.active == nil {
		return
	}
	cur := tab.active
	cx, cy := rectCenter(cur.bounds)
	var best *termPane
	bestScore := float32(1e9)
	tab.root.each(func(p *termPane) {
		if p == cur {
			return
		}
		x, y := rectCenter(p.bounds)
		dx, dy := x-cx, y-cy
		var along, across float32
		switch dir {
		case termFocusLeft:
			if dx >= 0 {
				return
			}
			along, across = -dx, abs(dy)
		case termFocusRight:
			if dx <= 0 {
				return
			}
			along, across = dx, abs(dy)
		case termFocusUp:
			if dy >= 0 {
				return
			}
			along, across = -dy, abs(dx)
		case termFocusDown:
			if dy <= 0 {
				return
			}
			along, across = dy, abs(dx)
		}
		if score := along + 2*across; score < bestScore {
			best, bestScore = p, score
		}
	})
	if best != nil {
		tab.active = best
		a.term.focus, a.term.focusReq = best, best
	}
}

// terminalShortcuts runs the keys a focused terminal takes, as Rex's
// window does, and reports whether one matched. The app's own keys with
// them are held back in shortcuts.
func (a *App) terminalShortcuts(c *ui.Context) bool {
	tp := &a.term
	if !tp.open || tp.view != panelTerminal {
		return false
	}
	switch {
	case c.Shortcut(ui.Cmd, ui.KeyT):
		a.newTerminalTab()
	case c.Shortcut(ui.Cmd, ui.KeyD):
		a.splitTerminal(false)
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyD):
		a.splitTerminal(true)
	case c.Shortcut(ui.Cmd, ui.KeyW):
		if tab := tp.active; tab != nil {
			a.closePane(tab.active)
		}
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyEnter):
		a.toggleTermZoom()
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyW):
		if tab := tp.active; tab != nil {
			a.closeTermTab(tab)
		}
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyR):
		if tab := tp.active; tab != nil {
			a.startTermRename(tab)
		}
	case c.Shortcut(ui.Cmd|ui.Ctrl, ui.KeyEqual):
		a.equalizeTermPanes()
	case c.Shortcut(ui.Cmd|ui.Alt, ui.KeyLeft):
		a.focusTermPane(termFocusLeft)
	case c.Shortcut(ui.Cmd|ui.Alt, ui.KeyRight):
		a.focusTermPane(termFocusRight)
	case c.Shortcut(ui.Cmd|ui.Alt, ui.KeyUp):
		a.focusTermPane(termFocusUp)
	case c.Shortcut(ui.Cmd|ui.Alt, ui.KeyDown):
		a.focusTermPane(termFocusDown)
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyBracketLeft):
		a.cycleTermTab(-1)
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyBracketRight):
		a.cycleTermTab(1)
	case c.Shortcut(ui.Cmd, ui.KeyK):
		if tab := tp.active; tab != nil && tab.active != nil && tab.active.term != nil {
			tab.active.term.ClearScreen()
		}
	case c.Shortcut(0, ui.KeyEscape):
		if tab := tp.active; tab != nil && tab.zoom != nil {
			a.toggleTermZoom()
		} else {
			return false
		}
	default:
		for i := 0; i < 9; i++ {
			if c.Shortcut(ui.Cmd, ui.Key1+ui.Key(i)) {
				a.activateTermTabNumber(i + 1)
				return true
			}
		}
		return false
	}
	return true
}

func rectCenter(r ui.Rect) (float32, float32) { return r.X + r.W/2, r.Y + r.H/2 }

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
