package config

// Workspace stores only navigation and presentation state; file contents
// remain on disk and are loaded through the normal document pipeline.
type Workspace struct {
	Folders        []WorkspaceFolder `json:"folders,omitempty"`
	Layout         *Layout           `json:"layout,omitempty"`
	SidebarHidden  bool              `json:"sidebarHidden,omitempty"`
	Navigator      int               `json:"navigator,omitempty"`
	RightNavigator *int              `json:"rightNavigator,omitempty"`
	// ChangesClosed, StashesClosed, and HistoryClosed are the Git
	// navigator's sections folded away.
	ChangesClosed   bool    `json:"changesClosed,omitempty"`
	StashesClosed   bool    `json:"stashesClosed,omitempty"`
	HistoryClosed   bool    `json:"historyClosed,omitempty"`
	GitSplit        float32 `json:"gitSplit,omitempty"`    // the changes' share of the Git navigator, as a percentage
	GitLogSplit     float32 `json:"gitLogSplit,omitempty"` // the commits' share of the Git log's tab, as a percentage
	WindowWidth     int     `json:"windowWidth,omitempty"`
	WindowHeight    int     `json:"windowHeight,omitempty"`
	RegexToolX      float32 `json:"regexToolX,omitempty"`
	RegexToolY      float32 `json:"regexToolY,omitempty"`
	RegexToolWidth  float32 `json:"regexToolWidth,omitempty"`
	RegexToolHeight float32 `json:"regexToolHeight,omitempty"`
	// Terminals are the panel's tabs, each its shells and their splits;
	// a shell's scrollback and command history are not kept.
	Terminals      []TermTab `json:"terminals,omitempty"`
	PanelOpen      bool      `json:"panelOpen,omitempty"`
	ActiveTerminal int       `json:"activeTerminal,omitempty"`
	// Bookmarks are the waypoints the workspace keeps, in the order the
	// user marked them.
	Bookmarks []Bookmark `json:"bookmarks,omitempty"`
}

// WorkspaceFolder is a root in a saved multi-root workspace.
type WorkspaceFolder struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

// TermTab is a terminal tab of the bottom panel: the panes the user
// named it for, its shells, the one that had the focus, and its splits.
type TermTab struct {
	Name   string     `json:"name,omitempty"`
	Panes  []TermPane `json:"panes,omitempty"`
	Active int        `json:"active,omitempty"`
	// Root is the tree the panes split in; sessions saved before it
	// laid them side by side.
	Root *TermLayout `json:"root,omitempty"`
	// Zoom is the pane the tab shows alone, as an index into Panes plus
	// one, or zero for none.
	Zoom int `json:"zoom,omitempty"`
}

// TermLayout is a node of a terminal tab's split tree: a pane, by its
// index in the tab's Panes, or two nodes split across.
type TermLayout struct {
	Pane     int         `json:"pane,omitempty"`
	Leaf     bool        `json:"leaf,omitempty"`
	Vertical bool        `json:"vertical,omitempty"`
	Ratio    float32     `json:"ratio,omitempty"`
	First    *TermLayout `json:"first,omitempty"`
	Second   *TermLayout `json:"second,omitempty"`
}

// TermPane is one shell of a terminal tab: the directory it worked in
// and the title its tab showed.
type TermPane struct {
	Dir   string `json:"dir,omitempty"`
	Title string `json:"title,omitempty"`
}

// Bookmark is a line of a file the workspace remembers: a path as the
// app's documents are keyed, and a line from 0.
type Bookmark struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

// WorkspaceSize is the window size a workspace remembers, or the default
// when it remembers none, or nonsense.
func WorkspaceSize(s Workspace) (int, int) {
	if s.WindowWidth < 480 || s.WindowHeight < 320 || s.WindowWidth > 10000 || s.WindowHeight > 10000 {
		return DefaultWindowWidth, DefaultWindowHeight
	}
	return s.WindowWidth, s.WindowHeight
}

// DefaultWindowWidth and DefaultWindowHeight are the window size of a
// workspace that remembers none.
const DefaultWindowWidth, DefaultWindowHeight = 1280, 820

type Layout struct {
	Tabs []Tab `json:"tabs,omitempty"`
	// Active is the active tab's path, or a Git tab's key in the app.
	Active        string  `json:"active,omitempty"`
	ActivePreview bool    `json:"activePreview,omitempty"`
	Preview       string  `json:"preview,omitempty"`
	Focused       bool    `json:"focused,omitempty"`
	Vertical      bool    `json:"vertical,omitempty"`
	Percent       float32 `json:"percent,omitempty"`
	First         *Layout `json:"first,omitempty"`
	Second        *Layout `json:"second,omitempty"`
}

type Tab struct {
	Folds           []int  `json:"folds"`
	Path            string `json:"path"`
	Untitled        bool   `json:"untitled,omitempty"`
	Name            string `json:"name,omitempty"`
	Line            int    `json:"line,omitempty"`
	Column          int    `json:"column,omitempty"`
	MarkdownPreview bool   `json:"markdownPreview,omitempty"`
	PreviewTab      bool   `json:"previewTab,omitempty"`
	// Diff is set for a tab of Git's, which Path is not a file of: a
	// change's diff, a commit, or the diff of a file of one.
	Diff *DiffTab `json:"diff,omitempty"`
}

// DiffTab is a tab of Git's, read again from the repository when the
// workspace opens: a change of the working tree or the index (Path,
// Staged, Untracked), a commit (Commit), or a file of one (Commit and
// Path, OldPath when renamed). Paths are the repository's, with slashes.
type DiffTab struct {
	Path      string `json:"path,omitempty"`
	OldPath   string `json:"oldPath,omitempty"`
	Staged    bool   `json:"staged,omitempty"`
	Untracked bool   `json:"untracked,omitempty"`
	Commit    string `json:"commit,omitempty"`
}
