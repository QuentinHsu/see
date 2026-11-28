package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

// A terminal's program shows as Rex's does: its name in the pane's
// header, and a tile of its color in the tab. Without a server to ask
// for the foreground process, the program is read from the title the
// terminal reports, and a title that is still the shell's is the shell.

// termProgram is how a program shows.
type termProgram struct {
	Name  string
	Glyph *ui.SVG
	// TileBg and TileFg color the tile the tab shows; TileBorder
	// outlines tiles too pale for the window.
	TileBg, TileFg ui.Color
	TileBorder     bool
	Shell          bool
}

var (
	tpTerminal = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="4" width="19" height="16" rx="3"/><path d="m7 9 3 3-3 3M13 15h4"/></svg>`))
	tpGlobe    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c2.5 2.5 3.8 5.7 3.8 9S14.5 18.5 12 21M12 3C9.5 5.5 8.2 8.7 8.2 12s1.3 6.5 3.8 9"/></svg>`))
	tpActivity = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12h4l2.5-6 4 12 2.5-6H21"/></svg>`))
	tpCode     = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="m8 7-5 5 5 5M16 7l5 5-5 5"/></svg>`))
	tpBox      = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="m12 2.8 8 4.6v9.2l-8 4.6-8-4.6V7.4l8-4.6Z"/><path d="m4.4 7.6 7.6 4.4 7.6-4.4M12 12v9.2"/></svg>`))
	tpCpu      = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><rect x="6" y="6" width="12" height="12" rx="2"/><rect x="9.5" y="9.5" width="5" height="5" rx="1"/><path d="M9 3v3M15 3v3M9 18v3M15 18v3M3 9h3M3 15h3M18 9h3M18 15h3"/></svg>`))
	tpDatabase = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><ellipse cx="12" cy="5.5" rx="8" ry="3"/><path d="M4 5.5v13c0 1.7 3.6 3 8 3s8-1.3 8-3v-13M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/></svg>`))
	tpDiff     = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round"><path d="M6 4v8M2 8h8M16 16h6M5 20h.01M5 15h.01M9 20h.01"/></svg>`))
	tpZoomIn   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M14 4h6v6M20 4l-6 6M10 20H4v-6M4 20l6-6"/></svg>`))
	tpAllPanes = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="3" width="8" height="8" rx="1.7"/><rect x="13" y="3" width="8" height="8" rx="1.7"/><rect x="3" y="13" width="8" height="8" rx="1.7"/><rect x="13" y="13" width="8" height="8" rx="1.7"/></svg>`))
)

var (
	tpShellTile = termProgram{Glyph: tpTerminal, TileBg: ui.Hex("#1d2420"), TileFg: ui.Hex("#5fd38d"), Shell: true}
	tpPrograms  = map[string]termProgram{
		"vim":       {Name: "Vim", Glyph: tpCode, TileBg: ui.Hex("#019733"), TileFg: ui.Hex("#ffffff")},
		"nvim":      {Name: "Neovim", Glyph: tpCode, TileBg: ui.Hex("#2b7a3d"), TileFg: ui.Hex("#ffffff")},
		"vi":        {Name: "Vim", Glyph: tpCode, TileBg: ui.Hex("#019733"), TileFg: ui.Hex("#ffffff")},
		"view":      {Name: "Vim", Glyph: tpCode, TileBg: ui.Hex("#019733"), TileFg: ui.Hex("#ffffff")},
		"emacs":     {Name: "Emacs", Glyph: tpCode, TileBg: ui.Hex("#7f5ab6"), TileFg: ui.Hex("#ffffff")},
		"hx":        {Name: "Helix", Glyph: tpCode, TileBg: ui.Hex("#281733"), TileFg: ui.Hex("#c7a3f5")},
		"lazygit":   {Name: "Git Changes", Glyph: tpDiff, TileBg: ui.Hex("#3f8f4f"), TileFg: ui.Hex("#ffffff")},
		"tig":       {Name: "Git Log", Glyph: icGit, TileBg: ui.Hex("#f05032"), TileFg: ui.Hex("#ffffff")},
		"git":       {Name: "Git", Glyph: icGit, TileBg: ui.Hex("#f05032"), TileFg: ui.Hex("#ffffff")},
		"node":      {Name: "Node", Glyph: tpBox, TileBg: ui.Hex("#5fa04e"), TileFg: ui.Hex("#ffffff")},
		"bun":       {Name: "Bun", Glyph: tpBox, TileBg: ui.Hex("#fbf0df"), TileFg: ui.Hex("#3b2a20"), TileBorder: true},
		"deno":      {Name: "Deno", Glyph: tpBox, TileBg: ui.Hex("#1b1b1b"), TileFg: ui.Hex("#ffffff")},
		"npm":       {Name: "npm", Glyph: tpBox, TileBg: ui.Hex("#cb3837"), TileFg: ui.Hex("#ffffff")},
		"pnpm":      {Name: "pnpm", Glyph: tpBox, TileBg: ui.Hex("#f69220"), TileFg: ui.Hex("#ffffff")},
		"yarn":      {Name: "Yarn", Glyph: tpBox, TileBg: ui.Hex("#2c8ebb"), TileFg: ui.Hex("#ffffff")},
		"python":    {Name: "Python", Glyph: tpCode, TileBg: ui.Hex("#3776ab"), TileFg: ui.Hex("#ffd43b")},
		"python3":   {Name: "Python", Glyph: tpCode, TileBg: ui.Hex("#3776ab"), TileFg: ui.Hex("#ffd43b")},
		"ipython":   {Name: "Python", Glyph: tpCode, TileBg: ui.Hex("#3776ab"), TileFg: ui.Hex("#ffd43b")},
		"ruby":      {Name: "Ruby", Glyph: tpCode, TileBg: ui.Hex("#cc342d"), TileFg: ui.Hex("#ffffff")},
		"irb":       {Name: "Ruby", Glyph: tpCode, TileBg: ui.Hex("#cc342d"), TileFg: ui.Hex("#ffffff")},
		"go":        {Name: "Go", Glyph: tpCode, TileBg: ui.Hex("#00add8"), TileFg: ui.Hex("#ffffff")},
		"cargo":     {Name: "Cargo", Glyph: tpCpu, TileBg: ui.Hex("#2b2b2b"), TileFg: ui.Hex("#f4a261")},
		"rustc":     {Name: "Rust", Glyph: tpCpu, TileBg: ui.Hex("#2b2b2b"), TileFg: ui.Hex("#f4a261")},
		"make":      {Name: "Make", Glyph: tpCpu, TileBg: ui.Hex("#6b7280"), TileFg: ui.Hex("#ffffff")},
		"docker":    {Name: "Docker", Glyph: tpBox, TileBg: ui.Hex("#2496ed"), TileFg: ui.Hex("#ffffff")},
		"ssh":       {Name: "SSH", Glyph: tpGlobe, TileBg: ui.Hex("#5b4bd6"), TileFg: ui.Hex("#ffffff")},
		"mosh":      {Name: "Mosh", Glyph: tpGlobe, TileBg: ui.Hex("#5b4bd6"), TileFg: ui.Hex("#ffffff")},
		"htop":      {Name: "Activity", Glyph: tpActivity, TileBg: ui.Hex("#1f6f43"), TileFg: ui.Hex("#a7f3c4")},
		"btop":      {Name: "Activity", Glyph: tpActivity, TileBg: ui.Hex("#1f6f43"), TileFg: ui.Hex("#a7f3c4")},
		"top":       {Name: "Activity", Glyph: tpActivity, TileBg: ui.Hex("#1f6f43"), TileFg: ui.Hex("#a7f3c4")},
		"tmux":      {Name: "tmux", Glyph: tpTerminal, TileBg: ui.Hex("#1bb91f"), TileFg: ui.Hex("#ffffff")},
		"psql":      {Name: "PostgreSQL", Glyph: tpDatabase, TileBg: ui.Hex("#336791"), TileFg: ui.Hex("#ffffff")},
		"mysql":     {Name: "MySQL", Glyph: tpDatabase, TileBg: ui.Hex("#00758f"), TileFg: ui.Hex("#ffffff")},
		"redis-cli": {Name: "Redis", Glyph: tpDatabase, TileBg: ui.Hex("#d82c20"), TileFg: ui.Hex("#ffffff")},
		"less":      {Name: "Pager", Glyph: tpCode, TileBg: ui.Hex("#475569"), TileFg: ui.Hex("#ffffff")},
		"man":       {Name: "Pager", Glyph: tpCode, TileBg: ui.Hex("#475569"), TileFg: ui.Hex("#ffffff")},
	}
)

// termProgramFor tells how a terminal's program shows, by the title the
// terminal reported and the shell a tab starts with.
func termProgramFor(title, shell string) termProgram {
	l := strings.ToLower(title)
	fields := strings.FieldsFunc(l, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_')
	})
	// The program a title ends with names it, as "file.go - NVIM" does;
	// earlier words may be the file's name, as "main.go".
	for i := len(fields) - 1; i >= 0; i-- {
		if p, ok := tpPrograms[fields[i]]; ok {
			return p
		}
	}
	if titleLooksLikeShell(title, shell) {
		p := tpShellTile
		p.Name = shell
		return p
	}
	name := strings.TrimSpace(title)
	if name == "" {
		p := tpShellTile
		p.Name = shell
		return p
	}
	return termProgram{Name: strings.ToValidUTF8(name, ""), Glyph: tpTerminal, TileBg: termHashColor(name), TileFg: ui.Hex("#ffffff")}
}

// titleLooksLikeShell tells a title a shell prints — the user and host,
// the directory — from one a program set.
func titleLooksLikeShell(title, shell string) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return true
	}
	l := strings.ToLower(t)
	if l == strings.ToLower(shell) {
		return true
	}
	if strings.HasPrefix(t, "~") || strings.HasPrefix(t, "/") || strings.HasPrefix(t, ".") {
		return true
	}
	if i := strings.Index(t, "@"); i > 0 {
		if j := strings.Index(t[i:], ":"); j > 0 {
			return true
		}
	}
	return false
}

// termHashColor picks a color for a name, the same every time.
func termHashColor(s string) ui.Color {
	palette := []string{"#6d5bd0", "#2f80ed", "#0f9d8a", "#d9822b", "#c2417d", "#3a7d44", "#8a5a44", "#4e5d94"}
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return ui.Hex(palette[h%uint32(len(palette))])
}

// termShortDir names a directory the way a tab has room for: its home as
// ~, a workspace folder as its name, and a deeper one by its last part.
func (a *App) termShortDir(dir string) string {
	if dir == "" {
		return ""
	}
	clean := filepath.Clean(dir)
	if home, err := os.UserHomeDir(); err == nil {
		if clean == home {
			return "~"
		}
		if rel, ok := strings.CutPrefix(clean, home+string(filepath.Separator)); ok {
			return "~/" + rel
		}
	}
	if a.root != "" {
		if rel, ok := strings.CutPrefix(clean, a.root+string(filepath.Separator)); ok {
			return rel
		}
		if clean == a.root {
			return filepath.Base(clean)
		}
	}
	return filepath.Base(clean)
}
