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
