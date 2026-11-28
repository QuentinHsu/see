package app

import (
	"github.com/egoist/mygo/ui"
	"see/locales"
)

// termTabsBar is the row under the panel's views: a tab for every
// terminal, as Rex lists them, and the button that opens another.
func (a *App) termTabsBar(c *ui.Context) {
	tp := &a.term
	bar := ui.Row(c).FillWidth().Height(termBarH).Shrink(0).Padding(0, 6).Gap(2).AlignItems(ui.Center).
		Background(colors(c).TabBar).BorderWidth(0, 0, 1, 0).BorderColor(colors(c).Hairline)
	bar.Children(func() {
		ui.ScrollHorizontal(c.Key("terminal-tabs")).Grow(1).MinWidth(0).FillHeight().Scrollbars(false).Children(func() {
			ui.Row(c).FillHeight().Gap(3).AlignItems(ui.Center).Children(func() {
				for _, tab := range tp.tabs {
					a.termTabItem(c, tab)
				}
			})
		})
		ui.Box(c).Size(1, 16).Shrink(0).Margin(0, 4).Background(colors(c).Hairline)
		if termIconButton(c, icPlus, commandLabel("terminal.new"), 24, 14).Clicked() {
			a.newTerminalTab()
		}
	})
}

// termTabDrag carries a tab the pointer drags across the strip.
type termTabDrag struct{ tab *termTab }

// termTabItem draws a terminal tab: the tiles of its panes' programs,
// its name and directory, and its close button or its dot.
func (a *App) termTabItem(c *ui.Context, tab *termTab) {
	tp := &a.term
	pal := colors(c)
	active := tp.active == tab
	name, detail := a.termTabLabel(tab)
	e := ui.Row(c.Key(tab)).Height(termTabH).Padding(0, 9, 0, 6).Gap(7).Radius(6).Shrink(0).
		MinWidth(104).MaxWidth(250).AlignItems(ui.Center).Label(name + " " + detail)
	switch {
	case active:
		e.Background(pal.TabActive).Border(1, pal.Hairline)
	case e.Hovered():
		e.Background(pal.TabHover)
	}
	e.Drag(&termTabDrag{tab: tab})
	if v, over := ui.DragOver[*termTabDrag](e); over && v.tab != tab {
		e.DrawOver(func(pt *ui.Painter, r ui.Rect) {
			pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: 2, H: r.H}, c.Theme().Accent, 1)
		})
	}
	if v, ok := ui.Drop[*termTabDrag](e); ok && v.tab != tab {
		a.moveTermTab(v.tab, a.termTabIndex(tab))
	}
	if e.Clicked() {
		if active {
			tp.focusReq = tab.active
			if tab.zoom != nil {
				tp.focusReq = tab.zoom
			}
		} else {
			a.selectTermTab(tab)
		}
	}
	if e.DoubleClicked() {
		a.selectTermTab(tab)
		a.startTermRename(tab)
	}
	e.ContextMenu(func(m *ui.Menu) { a.termTabMenu(m, tab) })
	hovered := e.Hovered()
	e.Children(func() {
		a.termTabTiles(c, tab)
		if tp.renaming == tab {
			a.termRenameField(c, tab)
			return
		}
		color := pal.Text
		if !active {
			color = pal.Muted
		}
		label := ui.Row(c).Grow(1).MinWidth(0).Gap(5).AlignItems(ui.Center)
		label.Children(func() {
			ui.Text(c, name).FontSize(12).FontWeight(500).TextColor(color).SingleLine().Ellipsis("…").Shrink(0)
			if detail != "" {
				ui.Text(c, detail).FontSize(11).TextColor(pal.Muted).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
			}
		})
		switch {
		case hovered && len(tp.tabs) > 1:
			b := ui.ButtonBase(c).Label(locales.MsgCloseTab()).Size(16, 16).Radius(3).Shrink(0).Center().KeepFocus()
			b.Children(func() { symbolIcon(c, icClose, 10, pal.Muted) })
			b.OnClick(func() { a.closeTermTab(tab) })
		case termTabAttention(tab):
			ui.Box(c).Size(7, 7).Shrink(0).Radius(4).Background(termAttentionColor(c))
		case a.termTabBusy(c, tab):
			ui.Box(c).Size(6, 6).Shrink(0).Radius(3).Background(termBusyColor(c))
		}
	})
}

// termTabLabel names a tab: the name the user gave it, or its focused
// pane's program and directory.
func (a *App) termTabLabel(tab *termTab) (name, detail string) {
	if tab.name != "" {
		return tab.name, ""
	}
	p := tab.active
	if tab.zoom != nil {
		p = tab.zoom
	}
	if p == nil {
		return a.shellTitle(), ""
	}
	return a.termPaneLabel(p, termProgramFor(p.title, a.shellTitle()))
}

// termTabAttention tells whether any pane of the tab rang or asked out
// of sight.
func termTabAttention(tab *termTab) bool {
	for _, p := range tab.panes() {
		if p.attention {
			return true
		}
	}
	return false
}

// termTabBusy tells whether any program of the tab is printing.
func (a *App) termTabBusy(c *ui.Context, tab *termTab) bool {
	for _, p := range tab.panes() {
		a.sampleTermPane(c, p)
		if a.termPaneStatus(c, p, termProgramFor(p.title, a.shellTitle())) == termBusy {
			return true
		}
	}
	return false
}

// termTabTiles draws the tiles of a tab's programs: its focused pane's
// in front, the others peeking out behind it.
func (a *App) termTabTiles(c *ui.Context, tab *termTab) {
	panes := tab.panes()
	front := tab.active
	if tab.zoom != nil {
		front = tab.zoom
	}
	var back []*termPane
	for _, p := range panes {
		if p != front {
			back = append(back, p)
		}
	}
	if len(back) > 2 {
		back = back[:2]
	}
	const tileW, tileH, step = 18, 14, 4
	w := float32(tileW + step*len(back) + 2)
	ui.Box(c).Size(w, tileH+4).Shrink(0).Children(func() {
		ui.Box(c).Size(w, tileH+4).Draw(func(pt *ui.Painter, r ui.Rect) {
			y := r.Y + 2
			for i := len(back) - 1; i >= 0; i-- {
				x := r.X + 1 + float32(step*(i+1))
				inset := float32(i+1) * 0.7
				a.drawTermTile(pt, ui.Rect{X: x, Y: y + inset, W: tileW, H: tileH - 2*inset}, back[i], false)
			}
			if front != nil {
				a.drawTermTile(pt, ui.Rect{X: r.X + 1, Y: y, W: tileW, H: tileH}, front, true)
			}
		})
	})
}

// drawTermTile draws a program's tile: a rounded card of its color, and
// its glyph when it is in front.
func (a *App) drawTermTile(pt *ui.Painter, r ui.Rect, p *termPane, glyph bool) {
	prog := termProgramFor(p.title, a.shellTitle())
	pt.Fill(r, prog.TileBg, 3.5)
	if prog.TileBorder {
		pt.Stroke(r, ui.RGBA(0, 0, 0, 0.12), 3.5, 0.5)
	}
	if !glyph || prog.Glyph == nil {
		return
	}
	const s = 10
	pt.Icon(prog.Glyph, ui.Rect{X: r.X + (r.W-s)/2, Y: r.Y + (r.H-s)/2, W: s, H: s}, prog.TileFg)
}

// termRenameField edits a tab's name in place: Enter or moving the focus
// away keeps it, Escape goes back, and an empty name follows the panes
// again.
func (a *App) termRenameField(c *ui.Context, tab *termTab) {
	tp := &a.term
	in := ui.TextInput(c.Key("terminal-rename"), &tp.rename).Placeholder(tab.name).Grow(1).MinWidth(0).
		Height(20).FontSize(12).AutoFocus()
	switch {
	case in.Shortcut(0, ui.KeyEscape):
		a.finishTermRename(false)
	case in.Submitted():
		a.finishTermRename(true)
	case in.Focused():
		tp.renameOff = true
	case tp.renameOff:
		a.finishTermRename(true)
	}
}

// termTabMenu is a tab's context menu.
func (a *App) termTabMenu(m *ui.Menu, tab *termTab) {
	if m.Item(locales.MsgRenameEllipsis()).Chosen() {
		a.selectTermTab(tab)
		a.startTermRename(tab)
	}
	if tab.name != "" && m.Item(locales.MsgResetName()).Chosen() {
		tab.name = ""
		a.saveConfigSoon()
	}
	m.Separator()
	i := a.termTabIndex(tab)
	if m.Item(locales.MsgMoveLeft()).Disabled(i <= 0).Chosen() {
		a.moveTermTab(tab, i-1)
	}
	if m.Item(locales.MsgMoveRight()).Disabled(i < 0 || i >= len(a.term.tabs)-1).Chosen() {
		a.moveTermTab(tab, i+1)
	}
	m.Separator()
	if m.Item(commandLabel("terminal.new")).Chosen() {
		a.newTerminalTab()
	}
	if m.Item(locales.MsgCloseTab()).Chosen() {
		a.closeTermTab(tab)
	}
	if m.Item(locales.MsgCloseOtherTabs()).Disabled(len(a.term.tabs) < 2).Chosen() {
		a.closeOtherTermTabs(tab)
	}
}
