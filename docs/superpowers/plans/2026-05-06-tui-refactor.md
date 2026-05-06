# TUI Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split `cmd/tui.go` (1957 lines) into 7 focused files in `package cmd`, move the setup wizard to a new `internal/setup` package, and add the brand header to the setup wizard view.

**Architecture:** All `package cmd` files share the same namespace so nothing needs re-exporting between them — this is a pure cut-and-paste split. `internal/setup` gets a clean API (`Run`, `ErrDeclined`) and receives `exampleKnotfile` and a `headerFn` closure from its caller in `package cmd`, keeping it free of cmd-specific dependencies.

**Tech Stack:** Go, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`

---

## File Map

| File | New / Modified | Contents |
|---|---|---|
| `cmd/tui.go` | Modified | `runTUI` only; imports `internal/setup`; drops wizard code |
| `cmd/tui_model.go` | **Create** | All `var` styles, all type/const defs, `model` struct, all `tea.Msg` types |
| `cmd/tui_mascot.go` | **Create** | `knotArt`, frame vars, `renderMascotLine` |
| `cmd/tui_rows.go` | **Create** | `pkgStatus`, `computeStatus`, row/tag building, toggle helpers |
| `cmd/tui_cmds.go` | **Create** | All `tea.Cmd` factories, `dotfilesDir` |
| `cmd/tui_update.go` | **Create** | `Init`, `Update`, all `updateXxx` handlers, `buildConfirmLines` |
| `cmd/tui_views.go` | **Create** | Layout helpers, `renderBrandHeader`, `View`, all `viewXxx` |
| `internal/setup/setup.go` | **Create** | Setup wizard: `Mode`, `ErrDeclined`, `Run`, `setupModel` and all its methods |

---

## Task 1: Verify Baseline

**Files:** Read-only

- [ ] **Step 1: Run tests and build**

```bash
go test ./cmd/... -count=1 && go build ./...
```

Expected: all tests pass, binary builds cleanly. Record the test count so you can confirm it doesn't change.

- [ ] **Step 2: Note tui.go line count**

```bash
wc -l cmd/tui.go
```

Expected: ~1957 lines. You'll use this to verify the file shrinks with each task.

---

## Task 2: Extract `cmd/tui_model.go`

Move all style vars, type definitions, the `model` struct, and all message types out of `tui.go` into a new file. No logic moves here — only declarations.

**Files:**
- Create: `cmd/tui_model.go`
- Modify: `cmd/tui.go` (delete moved lines)

- [ ] **Step 1: Create `cmd/tui_model.go`**

```go
package cmd

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
)

// ── layout constants ──────────────────────────────────────────────────────────

const (
	tuiMarginLeft  = 4
	tuiMarginRight = 4
)

// ── styles ────────────────────────────────────────────────────────────────────

var (
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleDim     = lipgloss.NewStyle().Faint(true)
	styleGreen   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRed     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleYellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleCursor  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	stylePending = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleMargin  = lipgloss.NewStyle().PaddingLeft(tuiMarginLeft).PaddingRight(tuiMarginRight)

	styleBorder = lipgloss.NewStyle().Foreground(lipgloss.Color("#c084fc"))
	styleArt    = [6]lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#e9d5ff")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#d8b4fe")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#c084fc")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#a855f7")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#9333ea")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#7e22ce")).Bold(true),
	}
	styleMascotNormal      = lipgloss.NewStyle().Foreground(lipgloss.Color("#fab387"))
	styleMascotConflict    = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	styleMascotMissing     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleMascotJellyNormal = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleMascotTentBlue    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	styleMascotTentRed     = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleMascotRobotNormal = lipgloss.NewStyle().Foreground(lipgloss.Color("#c084fc")).Bold(true)
)

// ── pkg status ────────────────────────────────────────────────────────────────

type pkgStatus int

const (
	statusUntied pkgStatus = iota
	statusTied
	statusPartial
	statusConflict
	statusSkipped
	statusSourceNotFound
)

const statusWidth = 9

// ── mascot enums ──────────────────────────────────────────────────────────────

type mascotState int

const (
	mascotNormal   mascotState = iota
	mascotConflict
	mascotMissing
)

type mascotCharacter int

const (
	mascotRobot     mascotCharacter = iota
	mascotJellyfish
	mascotMonkey
)

// ── row / tab types ───────────────────────────────────────────────────────────

type pkgRow struct {
	name    string
	status  pkgStatus
	actions []linker.LinkAction
}

type tagRow struct {
	name      string
	status    pkgStatus
	pkgs      []pkgRow
	collapsed bool
}

type tagItem struct {
	isTag       bool
	tag         *tagRow
	pkg         *pkgRow
	tagName     string
	isLastChild bool
}

type tabKind int

const (
	tabPackages tabKind = iota
	tabTags
)

// ── tui phases ────────────────────────────────────────────────────────────────

type tuiPhase int

const (
	phaseList tuiPhase = iota
	phaseConfirm
	phaseApply
	phaseResult
	phaseGitPull
	phaseBranch
	phaseCheckout
)

// ── model ─────────────────────────────────────────────────────────────────────

type model struct {
	cfg     *config.Config
	cfgPath string
	lnk     *linker.Linker

	rows    []pkgRow
	cursor  int
	offset  int
	toggles map[string]bool

	activeTab tabKind
	tagRows   []tagRow
	tagCursor int
	tagOffset int

	gitBranch    string
	gitSHA       string
	gitCommitMsg string

	branches     []string
	branchCursor int
	branchOffset int

	phase        tuiPhase
	confirmLines []string
	applyLog     []string
	applyErr     error
	statusMsg    string

	width, height int
	headerFrame   int
	mascotChar    mascotCharacter
}

// ── message types ─────────────────────────────────────────────────────────────

type headerTickMsg struct{}

type gitInfoMsg struct {
	branch string
	sha    string
	msg    string
	err    error
}

type gitPullResultMsg struct {
	output string
	err    error
}

type reloadMsg struct {
	rows []pkgRow
	cfg  *config.Config
	err  error
}

type applyDoneMsg struct {
	log []string
	err error
}

type editorDoneMsg struct{ err error }

type branchListMsg struct {
	branches []string
	err      error
}

type checkoutDoneMsg struct {
	output string
	err    error
}
```

- [ ] **Step 2: Delete the moved blocks from `cmd/tui.go`**

Remove these sections (in order from top of file):
  - The `const ( tuiMarginLeft … )` block (lines 23–26)
  - The entire `var ( styleBold … styleMascotRobotNormal … )` block (lines 28–56)
  - `type pkgStatus int` + its `const` block + `statusWidth` (lines 60–71)
  - `type mascotState int` + its `const` block (lines 145–150)
  - `type mascotCharacter int` + its `const` block (lines 153–159)
  - `type pkgRow struct` (lines 243–247)
  - `type tuiPhase int` + its `const` block (lines 249–260)
  - `type model struct` (lines 261–295)
  - All message type declarations: `headerTickMsg`, `gitInfoMsg`, `gitPullResultMsg`, `reloadMsg`, `applyDoneMsg`, `editorDoneMsg`, `branchListMsg`, `checkoutDoneMsg` (lines 299–336)
  - `type tabKind int` + its `const` block (lines 340–345)
  - `type tagRow struct` (lines 349–354)
  - `type tagItem struct` (lines 356–363)

Also remove the now-redundant imports `tea`, `lipgloss`, `linker` from `tui.go` **only if** they are no longer used there — do not remove an import that is still referenced in the remaining code in `tui.go`.

- [ ] **Step 3: Build to verify**

```bash
go build ./cmd/...
```

Expected: compiles cleanly (no undefined symbol errors).

- [ ] **Step 4: Run tests**

```bash
go test ./cmd/... -count=1
```

Expected: same pass count as Task 1.

- [ ] **Step 5: Commit**

```bash
git add cmd/tui.go cmd/tui_model.go
git commit -m "refactor(tui): extract tui_model.go"
```

---

## Task 3: Extract `cmd/tui_mascot.go`

**Files:**
- Create: `cmd/tui_mascot.go`
- Modify: `cmd/tui.go`

- [ ] **Step 1: Create `cmd/tui_mascot.go`**

```go
package cmd

import "strings"

// knotArt is "KNOT" in 6-row block-letter style; each row is 37 visual columns wide.
var knotArt = [6]string{
	`██╗  ██╗███╗   ██╗ ██████╗ ████████╗`,
	`██║ ██╔╝████╗  ██║██╔═══██╗╚══██╔══╝`,
	`█████╔╝ ██╔██╗ ██║██║   ██║   ██║   `,
	`██╔═██╗ ██║╚██╗██║██║   ██║   ██║   `,
	`██║  ██╗██║ ╚████║╚██████╔╝   ██║   `,
	`╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝    ╚═╝   `,
}

// monkeyFrames[state][frame][line] — each line is exactly 8 visual columns.
var monkeyFrames = [3][3][6]string{
	// mascotNormal: peach, blink (frame 1) + grin (frame 2)
	{
		{`  ▄▄▄▄  `, `▐(o  o)▌`, `▐( ▄▄ )▌`, `▐( ~~ )▌`, ` ▀(  )▀ `, `   ██   `},
		{`  ▄▄▄▄  `, `▐(─  ─)▌`, `▐( ▄▄ )▌`, `▐( ~~ )▌`, ` ▀(  )▀ `, `   ██   `},
		{`  ▄▄▄▄  `, `▐(o  o)▌`, `▐( ▄▄ )▌`, `▐( ^^ )▌`, ` ▀(  )▀ `, `   ██   `},
	},
	// mascotConflict: red, fur rises each frame + frantic eyes + bared teeth
	{
		{`  ▄▄▄▄  `, `▐(>  <)▌`, `▐( ▄▄ )▌`, `▐( !! )▌`, ` ▀(  )▀ `, `   ██   `},
		{` ▄▄▄▄▄▄ `, `▐(X  X)▌`, `▐( ▄▄ )▌`, `▐( ## )▌`, ` ▀(  )▀ `, `   ██   `},
		{`▄▄▄▄▄▄▄▄`, `▐(*  *)▌`, `▐( ▄▄ )▌`, `▐( >> )▌`, ` ▀(  )▀ `, `   ██   `},
	},
	// mascotMissing: yellow, eyes dart side-to-side
	{
		{`  ▄▄▄▄  `, `▐(o  .)▌`, `▐( ▄▄ )▌`, `▐( ?? )▌`, ` ▀(  )▀ `, `   ██   `},
		{`  ▄▄▄▄  `, `▐(.  .)▌`, `▐( ▄▄ )▌`, `▐( ?? )▌`, ` ▀(  )▀ `, `   ██   `},
		{`  ▄▄▄▄  `, `▐(.  o)▌`, `▐( ▄▄ )▌`, `▐( ?? )▌`, ` ▀(  )▀ `, `   ██   `},
	},
}

// jellyfishFrames[state][frame][line] — each line is exactly 8 visual columns.
// Tentacle lines use "|" as a colour-split marker (left=blue, right=red).
var jellyfishFrames = [3][3][6]string{
	{
		{` ▄████▄ `, `▐ o  o ▌`, `▐  ~~  ▌`, ` ▀████▀ `, `│╷│╷|║╿║╿`, `╵ ╵ |╵ ╵ `},
		{` ▄████▄ `, `▐ ─  ─ ▌`, `▐  ~~  ▌`, ` ▀████▀ `, `╵╷╵╷|╵╿╵╿`, `│ │ |║ ║ `},
		{` ▄████▄ `, `▐ o  o ▌`, `▐  ^^  ▌`, ` ▀████▀ `, `│╷│╷|║╿║╿`, `╵ ╵ |╵ ╵ `},
	},
	{
		{` ▄████▄ `, `▐ >  < ▌`, `▐  !!  ▌`, ` ▀████▀ `, `│╷│╷|║╿║╿`, `╵ ╵ |╵ ╵ `},
		{`▄██████▄`, `▐ X  X ▌`, `▐  ##  ▌`, ` ▀████▀ `, `╵╷╵╷|╵╿╵╿`, `│ │ |║ ║ `},
		{`████████`, `▐ *  * ▌`, `▐  >>  ▌`, ` ▀████▀ `, `│╷│╷|║╿║╿`, `╵ ╵ |╵ ╵ `},
	},
	{
		{` ▄████▄ `, `▐ o  . ▌`, `▐  ??  ▌`, ` ▀████▀ `, `╵╷╵╷|╵╿╵╿`, `│ │ |║ ║ `},
		{` ▄████▄ `, `▐ .  . ▌`, `▐  ??  ▌`, ` ▀████▀ `, `│╷│╷|║╿║╿`, `╵ ╵ |╵ ╵ `},
		{` ▄████▄ `, `▐ .  o ▌`, `▐  ??  ▌`, ` ▀████▀ `, `╵╷╵╷|╵╿╵╿`, `│ │ |║ ║ `},
	},
}

// robotFrames[state][frame][line] — each line is exactly 8 visual columns.
var robotFrames = [3][3][6]string{
	{
		{`████████`, `█      █`, `█ ▀  ▄ █`, `█      █`, `██    ██`, ` ██████ `},
		{`████████`, `█      █`, `█ ─  ─ █`, `█      █`, `██    ██`, ` ██████ `},
		{`████████`, `█      █`, `█ ▄  ▀ █`, `█      █`, `██    ██`, ` ██████ `},
	},
	{
		{`████████`, `█      █`, `█ >  < █`, `█      █`, `██    ██`, ` ██████ `},
		{`████████`, `█      █`, `█ X  X █`, `█      █`, `██    ██`, ` ██████ `},
		{`████████`, `█      █`, `█ *  * █`, `█      █`, `██    ██`, ` ██████ `},
	},
	{
		{`████████`, `█      █`, `█ o  . █`, `█      █`, `██    ██`, ` ██████ `},
		{`████████`, `█      █`, `█ .  . █`, `█      █`, `██    ██`, ` ██████ `},
		{`████████`, `█      █`, `█ .  o █`, `█      █`, `██    ██`, ` ██████ `},
	},
}

// renderMascotLine colours one mascot line. Lines containing "|" are split:
// left half → styleMascotTentBlue, right half → styleMascotTentRed.
func renderMascotLine(line string, bodyStyle lipgloss.Style) string {
	if idx := strings.Index(line, "|"); idx >= 0 {
		return styleMascotTentBlue.Render(line[:idx]) + styleMascotTentRed.Render(line[idx+1:])
	}
	return bodyStyle.Render(line)
}
```

Note: `renderMascotLine` references `styleMascotTentBlue` and `styleMascotTentRed` which are declared in `tui_model.go` — same package, no import needed. The `lipgloss.Style` parameter type requires the `lipgloss` import; add it to the import block.

Updated import for `tui_mascot.go`:
```go
import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)
```

- [ ] **Step 2: Delete the moved blocks from `cmd/tui.go`**

Remove:
  - `var knotArt = [6]string{…}` block
  - `var monkeyFrames = …` block
  - `var jellyfishFrames = …` block
  - `var robotFrames = …` block
  - `func renderMascotLine(…)` function

- [ ] **Step 3: Build and test**

```bash
go build ./cmd/... && go test ./cmd/... -count=1
```

Expected: clean build, same pass count.

- [ ] **Step 4: Commit**

```bash
git add cmd/tui.go cmd/tui_mascot.go
git commit -m "refactor(tui): extract tui_mascot.go"
```

---

## Task 4: Extract `cmd/tui_rows.go`

**Files:**
- Create: `cmd/tui_rows.go`
- Modify: `cmd/tui.go`

- [ ] **Step 1: Create `cmd/tui_rows.go`**

```go
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
)
```

Then move the following functions verbatim from `tui.go` into this file (bodies unchanged):
- `centerLabel(s string) string`
- `(s pkgStatus) label() string`
- `computeStatus(actions []linker.LinkAction) pkgStatus`
- `(m *model) pkgPendingArrow(row pkgRow) string`
- `(m *model) tagWouldBeWord(tr *tagRow) string`
- `(m *model) tagPendingArrow(tr *tagRow) string`
- `buildRows(cfg *config.Config, lnk *linker.Linker) ([]pkgRow, error)`
- `buildTagRows(cfg *config.Config, allRows []pkgRow) []tagRow`
- `visibleTagItems(rows []tagRow) []tagItem`
- `seedToggles(rows []pkgRow) map[string]bool`
- `(m *model) isPending(row pkgRow) bool`
- `(m *model) pendingCount() int`
- `(m *model) togglePackage(i int)`
- `(m *model) toggleTag(tr *tagRow)`

Check `tagWouldBeWord` — it uses `strings.Repeat` indirectly via `stylePending.Render`? Actually no: `tagWouldBeWord` just returns string literals. Check if `strings` is actually used in any moved function. `centerLabel` uses `strings.Repeat`. Keep the `strings` import.

- [ ] **Step 2: Delete the moved functions from `cmd/tui.go`**

- [ ] **Step 3: Build and test**

```bash
go build ./cmd/... && go test ./cmd/... -count=1
```

- [ ] **Step 4: Commit**

```bash
git add cmd/tui.go cmd/tui_rows.go
git commit -m "refactor(tui): extract tui_rows.go"
```

---

## Task 5: Extract `cmd/tui_cmds.go`

**Files:**
- Create: `cmd/tui_cmds.go`
- Modify: `cmd/tui.go`

- [ ] **Step 1: Create `cmd/tui_cmds.go`**

```go
package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
)
```

Move the following functions verbatim from `tui.go`:
- `dotfilesDir(cfgPath string) string`
- `headerTickCmd() tea.Cmd`
- `fetchGitInfoCmd(dir string) tea.Cmd`
- `fetchBranchesCmd(dir string) tea.Cmd`
- `checkoutBranchCmd(dir, branch string) tea.Cmd`
- `gitPullCmd(cfgPath string) tea.Cmd`
- `reloadCmd(cfgPath string, lnk *linker.Linker) tea.Cmd`
- `applyCmd(cfg *config.Config, lnk *linker.Linker, rows []pkgRow, toggles map[string]bool) tea.Cmd`
- `editorCmd(cfgPath string) tea.Cmd`

- [ ] **Step 2: Delete the moved functions from `cmd/tui.go`**

- [ ] **Step 3: Build and test**

```bash
go build ./cmd/... && go test ./cmd/... -count=1
```

- [ ] **Step 4: Commit**

```bash
git add cmd/tui.go cmd/tui_cmds.go
git commit -m "refactor(tui): extract tui_cmds.go"
```

---

## Task 6: Extract `cmd/tui_update.go`

**Files:**
- Create: `cmd/tui_update.go`
- Modify: `cmd/tui.go`

- [ ] **Step 1: Create `cmd/tui_update.go`**

```go
package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)
```

Move verbatim from `tui.go`:
- `(m model) Init() tea.Cmd`
- `(m model) Update(msg tea.Msg) (tea.Model, tea.Cmd)`
- `(m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd)`
- `(m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd)`
- `(m model) updateBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd)`
- `(m model) updateTags(msg tea.KeyMsg) (tea.Model, tea.Cmd)`
- `(m model) buildConfirmLines() []string`

- [ ] **Step 2: Delete the moved functions from `cmd/tui.go`**

- [ ] **Step 3: Build and test**

```bash
go build ./cmd/... && go test ./cmd/... -count=1
```

- [ ] **Step 4: Commit**

```bash
git add cmd/tui.go cmd/tui_update.go
git commit -m "refactor(tui): extract tui_update.go"
```

---

## Task 7: Extract `cmd/tui_views.go`

**Files:**
- Create: `cmd/tui_views.go`
- Modify: `cmd/tui.go`

- [ ] **Step 1: Create `cmd/tui_views.go`**

```go
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)
```

Move verbatim from `tui.go`:
- `(m *model) listHeaderLines() int`
- `(m *model) visibleHeight() int`
- `(m *model) adjustOffset()`
- `(m *model) branchVisibleHeight() int`
- `(m *model) adjustBranchOffset()`
- `(m *model) tagVisibleHeight() int`
- `(m *model) adjustTagOffset()`
- `(m model) renderBrandHeader() string`
- `(m model) hasConflicts() bool`
- `(m model) currentMascotState() mascotState`
- `(m model) View() string`
- `(m model) viewList() string`
- `(m model) viewTags() string`
- `(m model) viewBranch() string`
- `(m model) viewConfirm() string`
- `(m model) viewResult() string`

- [ ] **Step 2: Delete the moved functions from `cmd/tui.go`**

At this point `tui.go` should contain only: `runTUI`, `runSetupWizard`, `errSetupDeclined`, the setup wizard types (`setupMode`, `setupPhase`, `setupModel`, etc.), and `max`/`min`.

- [ ] **Step 3: Build and test**

```bash
go build ./cmd/... && go test ./cmd/... -count=1
```

- [ ] **Step 4: Commit**

```bash
git add cmd/tui.go cmd/tui_views.go
git commit -m "refactor(tui): extract tui_views.go"
```

---

## Task 8: Create `internal/setup` Package

Extract the entire setup wizard from `cmd/tui.go` into `internal/setup/setup.go` with a clean public API.

**Files:**
- Create: `internal/setup/setup.go`
- Modify: `cmd/tui.go`

- [ ] **Step 1: Create `internal/setup/setup.go`**

```go
package setup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oxgrad/knot/internal/config"
)

// ErrDeclined is returned by Run when the user explicitly chose not to create a Knotfile.
var ErrDeclined = errors.New("setup declined")

// Mode controls which wizard flow is shown.
type Mode int

const (
	ModeInit     Mode = iota // dotfiles dir missing: show init/clone menu
	ModeKnotfile             // dir present, Knotfile missing: confirm only
)

// Run runs the setup wizard TUI and returns nil on success, ErrDeclined if
// the user cancelled, or another error on failure.
// headerFn is called with the current terminal width to render the brand header.
// knotfileTemplate is written when creating a new Knotfile from scratch.
func Run(dir string, mode Mode, headerFn func(width int) string, knotfileTemplate []byte) error {
	phase := phaseMenu
	if mode == ModeKnotfile {
		phase = phaseConfirmKnotfile
	}
	m := model{
		dir:              dir,
		mode:             mode,
		phase:            phase,
		headerFn:         headerFn,
		knotfileTemplate: knotfileTemplate,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return err
	}
	sm, ok := final.(model)
	if !ok {
		return nil
	}
	if sm.err != nil {
		return sm.err
	}
	if sm.declined {
		return ErrDeclined
	}
	return nil
}

// ── internal types ────────────────────────────────────────────────────────────

type phase int

const (
	phaseMenu            phase = iota
	phaseGitProvider
	phaseGitProtocol
	phaseGitUsername
	phaseGitRepo
	phaseCloning
	phaseConfirmKnotfile
	phaseDone
)

type model struct {
	dir              string
	mode             Mode
	phase            phase
	cursor           int
	inputBuf         string
	gitProvider      string
	gitProtocol      string
	gitUsername      string
	gitRepo          string
	cloneURL         string
	err              error
	declined         bool
	width            int
	headerFn         func(width int) string
	knotfileTemplate []byte
}

// ── message types ─────────────────────────────────────────────────────────────

type cloneDoneMsg struct{ err error }
type knotfileReadyMsg struct{ err error }

// ── tea interface ─────────────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	if m.mode == ModeKnotfile {
		return func() tea.Msg { return nil }
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case cloneDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.phase = phaseGitRepo
			return m, nil
		}
		knotfilePath := filepath.Join(m.dir, config.KnotfileName)
		if _, err := os.Stat(knotfilePath); os.IsNotExist(err) {
			return m, writeKnotfileCmd(m.dir, m.knotfileTemplate)
		}
		m.phase = phaseDone
		return m, tea.Quit

	case knotfileReadyMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.phase = phaseDone
		return m, tea.Quit

	case tea.KeyMsg:
		switch m.phase {
		case phaseMenu:
			return m.updateMenu(msg)
		case phaseGitProvider:
			return m.updateGitProvider(msg)
		case phaseGitProtocol:
			return m.updateGitProtocol(msg)
		case phaseGitUsername:
			return m.updateGitInput(msg, phaseGitProtocol, phaseGitRepo)
		case phaseGitRepo:
			return m.updateGitInput(msg, phaseGitUsername, phaseGitRepo)
		case phaseConfirmKnotfile:
			return m.updateConfirmKnotfile(msg)
		}
	}
	return m, nil
}

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter", " ":
		m.err = nil
		if m.cursor == 0 {
			return m, writeKnotfileCmd(m.dir, m.knotfileTemplate)
		}
		m.phase = phaseGitProvider
		m.cursor = 0
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateGitProvider(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter", " ":
		if m.cursor == 0 {
			m.gitProvider = "github"
		} else {
			m.gitProvider = "gitlab"
		}
		m.cursor = 0
		m.phase = phaseGitProtocol
	case "esc":
		m.phase = phaseMenu
		m.cursor = 1
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateGitProtocol(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter", " ":
		if m.cursor == 0 {
			m.gitProtocol = "https"
		} else {
			m.gitProtocol = "ssh"
		}
		m.cursor = 0
		m.inputBuf = ""
		m.phase = phaseGitUsername
	case "esc":
		m.phase = phaseGitProvider
		m.cursor = 0
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateGitInput(msg tea.KeyMsg, backPhase, nextPhase phase) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		m.inputBuf += msg.String()
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.inputBuf) > 0 {
			runes := []rune(m.inputBuf)
			m.inputBuf = string(runes[:len(runes)-1])
		}
	case tea.KeyEnter:
		if m.phase == phaseGitUsername {
			username := strings.TrimSpace(m.inputBuf)
			if username == "" {
				return m, nil
			}
			m.gitUsername = username
			m.inputBuf = ""
			m.phase = phaseGitRepo
		} else {
			repo := strings.TrimSpace(m.inputBuf)
			if repo == "" {
				repo = ".dotfiles"
			}
			m.gitRepo = repo
			m.cloneURL = buildGitURL(m.gitProvider, m.gitProtocol, m.gitUsername, repo)
			m.err = nil
			m.phase = phaseCloning
			return m, cloneRepoCmd(m.cloneURL, m.dir)
		}
	case tea.KeyEsc:
		m.inputBuf = ""
		m.err = nil
		m.phase = backPhase
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	_ = nextPhase
	return m, nil
}

func (m model) updateConfirmKnotfile(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		return m, writeKnotfileCmd(m.dir, m.knotfileTemplate)
	case "n", "esc", "q", "ctrl+c":
		m.declined = true
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	if m.headerFn != nil {
		b.WriteString(m.headerFn(m.width))
	}

	b.WriteString(boldStyle.Render("knot setup") + "\n\n")

	switch m.phase {
	case phaseMenu:
		b.WriteString("No dotfiles directory found at " + cyanStyle.Render(m.dir) + ".\n\n")
		opts := []string{"Initialize new dotfiles folder", "Clone existing dotfiles from git"}
		for i, opt := range opts {
			if i == m.cursor {
				b.WriteString(cursorStyle.Render("> ") + boldStyle.Render(opt) + "\n")
			} else {
				b.WriteString("  " + opt + "\n")
			}
		}
		b.WriteString("\n" + dimStyle.Render("↑/↓ to move · enter to select · q to quit"))

	case phaseGitProvider:
		b.WriteString("Select a git provider:\n\n")
		providers := []string{"GitHub", "GitLab"}
		for i, p := range providers {
			if i == m.cursor {
				b.WriteString(cursorStyle.Render("> ") + boldStyle.Render(p) + "\n")
			} else {
				b.WriteString("  " + p + "\n")
			}
		}
		b.WriteString("\n" + dimStyle.Render("↑/↓ to move · enter to select · esc to go back"))

	case phaseGitProtocol:
		host := "github.com"
		if m.gitProvider == "gitlab" {
			host = "gitlab.com"
		}
		b.WriteString("Select a protocol for " + cyanStyle.Render(host) + ":\n\n")
		protocols := []string{"HTTPS", "SSH"}
		for i, p := range protocols {
			if i == m.cursor {
				b.WriteString(cursorStyle.Render("> ") + boldStyle.Render(p) + "\n")
			} else {
				b.WriteString("  " + p + "\n")
			}
		}
		b.WriteString("\n" + dimStyle.Render("↑/↓ to move · enter to select · esc to go back"))
		b.WriteString("\n" + dimStyle.Render("Note: SSH requires your public key to be added to your "+host+" account."))

	case phaseGitUsername:
		b.WriteString("Enter your " + cyanStyle.Render(m.gitProvider) + " username:\n\n")
		b.WriteString("  " + cyanStyle.Render(m.inputBuf) + "█\n")
		b.WriteString("\n" + dimStyle.Render("enter to confirm · esc to go back · ctrl+c to quit"))

	case phaseGitRepo:
		b.WriteString("Enter the repository name:\n\n")
		b.WriteString("  " + cyanStyle.Render(m.inputBuf) + "█\n")
		b.WriteString("\n" + dimStyle.Render("enter to confirm · esc to go back · ctrl+c to quit"))
		b.WriteString("\n" + dimStyle.Render("Leave empty to use the default: ") + cyanStyle.Render(".dotfiles"))
		if m.err != nil {
			b.WriteString("\n\n" + redStyle.Render(m.err.Error()))
		}

	case phaseCloning:
		b.WriteString("Cloning " + cyanStyle.Render(m.cloneURL) + "\n")
		b.WriteString("into    " + cyanStyle.Render(m.dir) + "\n\n")
		b.WriteString(dimStyle.Render("Please wait…"))

	case phaseConfirmKnotfile:
		b.WriteString("No Knotfile found in " + cyanStyle.Render(m.dir) + ".\n\n")
		b.WriteString("Create one from template? " + boldStyle.Render("[y/n]") + "\n")
		if m.err != nil {
			b.WriteString("\n" + redStyle.Render(m.err.Error()))
		}

	case phaseDone:
		b.WriteString(greenStyle.Render("Setup complete.") + " Starting knot…")
	}

	return b.String()
}

// ── local styles ──────────────────────────────────────────────────────────────

var (
	boldStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Faint(true)
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
)

Also add the two cmd helper functions, moved verbatim:

```go
func buildGitURL(provider, protocol, username, repo string) string {
	if repo == "" {
		repo = ".dotfiles"
	}
	switch {
	case provider == "github" && protocol == "https":
		return fmt.Sprintf("https://github.com/%s/%s", username, repo)
	case provider == "github" && protocol == "ssh":
		return fmt.Sprintf("git@github.com:%s/%s.git", username, repo)
	case provider == "gitlab" && protocol == "https":
		return fmt.Sprintf("https://gitlab.com/%s/%s", username, repo)
	case provider == "gitlab" && protocol == "ssh":
		return fmt.Sprintf("git@gitlab.com:%s/%s.git", username, repo)
	}
	return ""
}

func cloneRepoCmd(url, dir string) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("git", "clone", url, dir)
		if err := c.Run(); err != nil {
			return cloneDoneMsg{err: fmt.Errorf("git clone failed: %w", err)}
		}
		return cloneDoneMsg{}
	}
}

func writeKnotfileCmd(dir string, template []byte) tea.Cmd {
	return func() tea.Msg {
		knotfilePath := filepath.Join(dir, config.KnotfileName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return knotfileReadyMsg{err: fmt.Errorf("creating directory: %w", err)}
		}
		if err := os.WriteFile(knotfilePath, template, 0o644); err != nil {
			return knotfileReadyMsg{err: fmt.Errorf("writing Knotfile: %w", err)}
		}
		return knotfileReadyMsg{}
	}
}
```

- [ ] **Step 2: Delete wizard code from `cmd/tui.go`**

Remove from `cmd/tui.go`:
  - `type setupMode int` + its `const` block
  - `type setupPhase int` + its `const` block
  - `type setupModel struct`
  - `func buildGitURL(…)`
  - `type ( cloneDoneMsg … knotfileReadyMsg … )`
  - `func cloneRepoCmd(…)`
  - `func writeKnotfileCmd(…)` — the version that uses `exampleKnotfile`
  - `func (m setupModel) Init()`
  - `func (m setupModel) Update(…)`
  - All `updateMenu`, `updateGitProvider`, `updateGitProtocol`, `updateGitInput`, `updateConfirmKnotfile` methods on `setupModel`
  - `func (m setupModel) View()`
  - `var errSetupDeclined = …`
  - `func runSetupWizard(…)` — will be replaced in the next task

- [ ] **Step 3: Update `go.mod` if needed**

```bash
go mod tidy
```

- [ ] **Step 4: Build and test**

```bash
go build ./... && go test ./cmd/... -count=1
```

Expected: compiles cleanly. (`runTUI` still calls `runSetupWizard` which no longer exists — the build will fail until Task 9 wires things up. That's expected. If you want a green build at this step, add a temporary stub in `tui.go`:

```go
func runSetupWizard(dir string, mode int) error { return nil }
```

Remove the stub in Task 9.)

- [ ] **Step 5: Commit**

```bash
git add cmd/tui.go internal/setup/setup.go go.mod go.sum
git commit -m "refactor(tui): extract internal/setup package"
```

---

## Task 9: Wire `setup.Run` into `cmd/tui.go` + Brand Header

Replace the deleted `runSetupWizard` + `errSetupDeclined` with direct calls to `setup.Run`, passing the brand header closure and `exampleKnotfile`.

**Files:**
- Modify: `cmd/tui.go`

- [ ] **Step 1: Add `internal/setup` import to `cmd/tui.go`**

The import block in `tui.go` should become:

```go
import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
	"github.com/oxgrad/knot/internal/setup"
)
```

- [ ] **Step 2: Replace `runSetupWizard` calls in `runTUI`**

Find the two calls to `runSetupWizard` inside `runTUI` and replace them:

```go
// Before (remove):
if wizErr := runSetupWizard(dir, setupModeInit); wizErr != nil {
    return wizErr
}

// After:
if wizErr := setup.Run(dir, setup.ModeInit, brandHeaderFn, exampleKnotfile); wizErr != nil {
    return wizErr
}
```

```go
// Before (remove):
wizErr := runSetupWizard(dir, setupModeKnotfile)
if wizErr == errSetupDeclined {

// After:
wizErr := setup.Run(dir, setup.ModeKnotfile, brandHeaderFn, exampleKnotfile)
if errors.Is(wizErr, setup.ErrDeclined) {
```

- [ ] **Step 3: Add `brandHeaderFn` helper to `cmd/tui.go`**

Add this function just before `runTUI`:

```go
// brandHeaderFn returns a closure that renders the brand header at the given
// terminal width. Used to inject the header into the setup wizard.
func brandHeaderFn(width int) string {
	return model{width: width}.renderBrandHeader()
}
```

- [ ] **Step 4: Remove the temporary stub if you added one in Task 8**

Delete any `func runSetupWizard(…)` stub.

- [ ] **Step 5: Build and test**

```bash
go build ./... && go test ./cmd/... -count=1
```

Expected: clean build, all tests pass.

- [ ] **Step 6: Smoke-test the wizard header manually**

```bash
go run . 2>/dev/null || go run ./main.go
```

Open in a terminal that has no `~/.dotfiles` dir (or rename it temporarily). Confirm the brand header renders at the top of the setup wizard screen.

- [ ] **Step 7: Commit**

```bash
git add cmd/tui.go
git commit -m "feat(setup): wire setup.Run with brand header injection"
```

---

## Task 10: Final Cleanup and Verification

- [ ] **Step 1: Verify `cmd/tui.go` contains only what it should**

```bash
grep -n "^func\|^type\|^var\|^const" cmd/tui.go
```

Expected output (roughly):
```
func brandHeaderFn(width int) string
func runTUI(cmd *cobra.Command, args []string) error
func max(a, b int) int
func min(a, b int) int
```

- [ ] **Step 2: Check line counts**

```bash
wc -l cmd/tui.go cmd/tui_model.go cmd/tui_mascot.go cmd/tui_rows.go cmd/tui_cmds.go cmd/tui_update.go cmd/tui_views.go internal/setup/setup.go
```

`cmd/tui.go` should be under 80 lines. No single file should exceed 350 lines.

- [ ] **Step 3: Confirm tests still pass with race detector**

```bash
go test -race ./... -count=1
```

Expected: all pass, no races.

- [ ] **Step 4: Confirm `go vet` is clean**

```bash
go vet ./...
```

Expected: no output.

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "refactor(tui): final cleanup — tui.go reduced to entry point"
```
