package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── layout helpers ────────────────────────────────────────────────────────────

func (m *model) listHeaderLines() int {
	// blank + brand box (13 lines, tabs embedded in bottom border) = 14
	return 14
}

func (m *model) visibleHeight() int {
	overhead := m.listHeaderLines() + 6
	v := m.height - overhead
	if v < 1 {
		return 1
	}
	return v
}

func (m *model) adjustOffset() {
	visibleRows := m.visibleHeight()
	if visibleRows <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleRows {
		m.offset = m.cursor - visibleRows + 1
	}
}

func (m *model) branchVisibleHeight() int {
	// title + divider + blank + help = 4
	v := m.height - 4
	if v < 1 {
		return 1
	}
	return v
}

func (m *model) adjustBranchOffset() {
	vh := m.branchVisibleHeight()
	if vh <= 0 {
		return
	}
	if m.branchCursor < m.branchOffset {
		m.branchOffset = m.branchCursor
	}
	if m.branchCursor >= m.branchOffset+vh {
		m.branchOffset = m.branchCursor - vh + 1
	}
}

func (m *model) tagVisibleHeight() int {
	overhead := m.listHeaderLines() + 6
	v := m.height - overhead
	if v < 1 {
		return 1
	}
	return v
}

func (m *model) adjustTagOffset() {
	items := visibleTagItems(m.tagRows)
	vh := m.tagVisibleHeight()
	if vh <= 0 || len(items) == 0 {
		return
	}
	if m.tagCursor < m.tagOffset {
		m.tagOffset = m.tagCursor
	}
	if m.tagCursor >= m.tagOffset+vh {
		m.tagOffset = m.tagCursor - vh + 1
	}
	maxOffset := max(0, len(items)-vh)
	if m.tagOffset > maxOffset {
		m.tagOffset = maxOffset
	}
}

// ── brand header ──────────────────────────────────────────────────────────────

func (m model) hasConflicts() bool {
	for _, r := range m.rows {
		if r.status == statusConflict {
			return true
		}
	}
	return false
}

func (m model) currentMascotState() mascotState {
	if m.hasConflicts() {
		return mascotConflict
	}
	if len(m.rows) == 0 || m.gitBranch == "" {
		return mascotMissing
	}
	return mascotNormal
}

func (m model) renderBrandHeader() string {
	state := m.currentMascotState()
	var frame int
	if state == mascotNormal {
		frame = (m.headerFrame / 2) % 3
	} else {
		frame = m.headerFrame % 3
	}
	var frames [3][3][6]string
	switch m.mascotChar {
	case mascotMonkey:
		frames = monkeyFrames
	case mascotRobot:
		frames = robotFrames
	default:
		frames = jellyfishFrames
	}
	mascotLines := frames[state][frame]

	var mascotStyle lipgloss.Style
	switch state {
	case mascotConflict:
		mascotStyle = styleMascotConflict
	case mascotMissing:
		mascotStyle = styleMascotMissing
	default:
		switch m.mascotChar {
		case mascotRobot:
			mascotStyle = styleMascotRobotNormal
		case mascotJellyfish:
			mascotStyle = styleMascotJellyNormal
		default:
			mascotStyle = styleMascotNormal
		}
	}

	const gap = 4
	// left section: 4 spaces + art (37) + gap (4) + mascot (8) + 3 padding = 56
	const divX = 56

	innerW := max(m.width-tuiMarginLeft-tuiMarginRight, 62) - 2
	rightW := max(innerW-divX-1, 0)

	var b strings.Builder

	b.WriteString("\n")
	titleName := styleBorder.Render("Knot")
	titleVersion := styleDim.Render(" v" + Version)
	titleSegment := " " + titleName + titleVersion + " "
	titleSegmentW := lipgloss.Width(titleSegment)
	leftTopFill := styleBorder.Render("──") + titleSegment + styleBorder.Render(strings.Repeat("─", max(divX-2-titleSegmentW, 0)))
	rightTopFill := styleBorder.Render(strings.Repeat("─", rightW))
	b.WriteString(styleBorder.Render("╭") + leftTopFill + styleBorder.Render("┬") + rightTopFill + styleBorder.Render("╮") + "\n")

	rightRows := make([]string, 11)
	fill := func(s string) string { return s + strings.Repeat(" ", max(rightW-lipgloss.Width(s), 0)) }
	for i := range rightRows {
		rightRows[i] = strings.Repeat(" ", rightW)
	}
	if m.gitBranch != "" {
		rightRows[4] = fill(" " + styleDim.Render("branch  ") + styleCyan.Render(m.gitBranch))
	}
	if m.gitSHA != "" {
		rightRows[5] = fill(" " + styleDim.Render("commit  ") + styleDim.Render(m.gitSHA))
	}
	if m.gitCommitMsg != "" {
		maxMsgLen := max(rightW-len(" message  ")-1, 5)
		msg := []rune(m.gitCommitMsg)
		if len(msg) > maxMsgLen {
			msg = append(msg[:maxMsgLen-1], '…')
		}
		rightRows[6] = fill(" " + styleDim.Render("message ") + string(msg))
	}
	if m.phase == phaseGitPull {
		heartbeat := [3]string{"·", "●", "·"}
		dot := heartbeat[m.headerFrame%3]
		rightRows[7] = fill(" " + styleCyan.Render(dot) + " " + styleDim.Render("pulling ") + styleDim.Render(dotfilesDir(m.cfgPath)+"..."))
	}

	writeRow := func(left, right string) {
		b.WriteString(styleBorder.Render("│") + left + styleBorder.Render("│") + right + styleBorder.Render("│") + "\n")
	}

	writeRow(strings.Repeat(" ", divX), rightRows[0])
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("LOGNAME")
	}
	welcomeLeft := "    " + styleDim.Render("Welcome, ") + username + "!"
	welcomeLeft += strings.Repeat(" ", max(divX-lipgloss.Width(welcomeLeft), 0))
	writeRow(welcomeLeft, rightRows[1])
	writeRow(strings.Repeat(" ", divX), rightRows[2])
	for i := 0; i < 6; i++ {
		art := styleArt[i].Render(knotArt[i])
		mascot := renderMascotLine(mascotLines[i], mascotStyle)
		leftContent := "    " + art + strings.Repeat(" ", gap) + mascot
		leftContent += strings.Repeat(" ", max(divX-lipgloss.Width(leftContent), 0))
		writeRow(leftContent, rightRows[3+i])
	}
	writeRow(strings.Repeat(" ", divX), rightRows[9])
	writeRow(strings.Repeat(" ", divX), rightRows[10])

	var tabSegment string
	if m.phase == phaseConfirm {
		tabSegment = " " + styleBold.Render("Pending changes") + "  "
	} else {
		var pkgTab, tagTab string
		if m.activeTab == tabPackages {
			pkgTab = styleBold.Render("Packages")
			tagTab = styleDim.Render("Tags")
		} else {
			pkgTab = styleDim.Render("Packages")
			tagTab = styleBold.Render("Tags")
		}
		tabSegment = " " + pkgTab + styleDim.Render(" · ") + tagTab + "  "
	}
	tabSegmentW := lipgloss.Width(tabSegment)
	leftBottomFill := styleBorder.Render("── ") + tabSegment + styleBorder.Render(strings.Repeat("─", max(divX-3-tabSegmentW, 0)))
	b.WriteString(styleBorder.Render("╰") + leftBottomFill + styleBorder.Render("┴") + styleBorder.Render(strings.Repeat("─", rightW)) + styleBorder.Render("╯") + "\n")

	return b.String()
}

// ── views ─────────────────────────────────────────────────────────────────────

func (m model) View() string {
	var v string
	switch m.phase {
	case phaseConfirm:
		v = m.viewConfirm()
	case phaseApply:
		v = styleDim.Render("Applying changes...")
	case phaseResult:
		v = m.viewResult()
	case phaseCheckout:
		v = styleDim.Render(fmt.Sprintf("Switching branch in %s...", dotfilesDir(m.cfgPath)))
	case phaseBranch:
		v = m.viewBranch()
	default:
		if m.activeTab == tabTags {
			v = m.viewTags()
		} else {
			v = m.viewList()
		}
	}
	return styleMargin.Render(v)
}

func (m model) viewList() string {
	var b strings.Builder

	b.WriteString(m.renderBrandHeader())
	b.WriteString("\n")

	if len(m.rows) == 0 {
		b.WriteString(styleDim.Render("No packages defined in Knotfile.") + "\n")
	} else {
		nameWidth := 0
		for _, r := range m.rows {
			if len(r.name) > nameWidth {
				nameWidth = len(r.name)
			}
		}

		visibleRows := m.visibleHeight()
		end := m.offset + visibleRows
		if end > len(m.rows) {
			end = len(m.rows)
		}

		if m.offset > 0 {
			b.WriteString(styleDim.Render(fmt.Sprintf("  ↑ %d more", m.offset)) + "\n")
		} else {
			b.WriteString("\n")
		}

		for i := m.offset; i < end; i++ {
			row := m.rows[i]

			cursor := "  "
			if i == m.cursor {
				cursor = styleCursor.Render("▶ ")
			}

			name := fmt.Sprintf("%-*s", nameWidth, row.name)
			pending := m.pkgPendingArrow(row)
			fmt.Fprintf(&b, "%s  %s  [%s]%s\n", cursor, name, row.status.label(), pending)
		}

		below := len(m.rows) - end
		if below > 0 {
			b.WriteString(styleDim.Render(fmt.Sprintf("  ↓ %d more", below)) + "\n")
		} else {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	if m.statusMsg != "" {
		b.WriteString(styleRed.Render(m.statusMsg) + "\n")
	} else if pc := m.pendingCount(); pc > 0 {
		b.WriteString(styleYellow.Render(fmt.Sprintf("%d pending change(s)", pc)) + "\n")
	} else {
		b.WriteString(styleDim.Render("No pending changes") + "\n")
	}

	b.WriteString(styleDim.Render("↑↓/jk navigate · space toggle · a apply · b branch · r pull · e edit · q quit"))

	return b.String()
}

func (m model) viewTags() string {
	var b strings.Builder

	b.WriteString(m.renderBrandHeader())
	b.WriteString("\n")

	items := visibleTagItems(m.tagRows)
	if len(items) == 0 {
		b.WriteString(styleDim.Render("No tagged packages defined.") + "\n")
	} else {
		nameWidth := 0
		for _, tr := range m.tagRows {
			if len(tr.name) > nameWidth {
				nameWidth = len(tr.name)
			}
			for _, pkg := range tr.pkgs {
				// indent + connector = 7 chars ("  ├── " or "  └── ")
				if len(pkg.name)+7 > nameWidth {
					nameWidth = len(pkg.name) + 7
				}
			}
		}

		vh := m.tagVisibleHeight()
		end := m.tagOffset + vh
		if end > len(items) {
			end = len(items)
		}

		if m.tagOffset > 0 {
			b.WriteString(styleDim.Render(fmt.Sprintf("  ↑ %d more", m.tagOffset)) + "\n")
		} else {
			b.WriteString("\n")
		}

		for i := m.tagOffset; i < end; i++ {
			item := items[i]
			cursor := "  "
			if i == m.tagCursor {
				cursor = styleCursor.Render("▶ ")
			}

			if item.isTag {
				collapsePrefix := "  "
				if item.tag.collapsed {
					collapsePrefix = styleDim.Render("▶ ")
				}
				pendingMark := m.tagPendingArrow(item.tag)
				name := fmt.Sprintf("%-*s", nameWidth, item.tag.name)
				fmt.Fprintf(&b, "%s%s%s  [%s]%s\n",
					cursor, collapsePrefix,
					styleCyan.Render(styleBold.Render(name)),
					item.tag.status.label(), pendingMark)
			} else {
				connector := "├── "
				if item.isLastChild {
					connector = "└── "
				}
				pkgName := fmt.Sprintf("%-*s", nameWidth-7, item.pkg.name)
				pendingMark := m.pkgPendingArrow(*item.pkg)
				fmt.Fprintf(&b, "%s  %s  [%s]%s\n",
					cursor,
					styleDim.Render(connector+pkgName),
					item.pkg.status.label(), pendingMark)
			}
		}

		below := len(items) - end
		if below > 0 {
			b.WriteString(styleDim.Render(fmt.Sprintf("  ↓ %d more", below)) + "\n")
		} else {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString(styleRed.Render(m.statusMsg) + "\n")
	} else if pc := m.pendingCount(); pc > 0 {
		b.WriteString(styleYellow.Render(fmt.Sprintf("%d pending change(s)", pc)) + "\n")
	} else {
		b.WriteString(styleDim.Render("No pending changes") + "\n")
	}
	b.WriteString(styleDim.Render("↑↓/jk navigate · space toggle · enter collapse · a apply · r pull · [ ] tabs · q quit"))
	return b.String()
}

func (m model) viewBranch() string {
	var b strings.Builder

	title := styleBold.Render("Switch branch")
	if m.gitBranch != "" {
		title += styleDim.Render("  (on ") + styleCyan.Render(m.gitBranch) + styleDim.Render(")")
	}
	b.WriteString(title + "\n")
	b.WriteString(strings.Repeat("─", max(m.width-tuiMarginLeft-tuiMarginRight, 30)) + "\n")

	if len(m.branches) == 0 {
		b.WriteString(styleDim.Render("No branches found.") + "\n")
	} else {
		nameWidth := 0
		for _, br := range m.branches {
			if len(br) > nameWidth {
				nameWidth = len(br)
			}
		}

		vh := m.branchVisibleHeight()
		end := m.branchOffset + vh
		if end > len(m.branches) {
			end = len(m.branches)
		}

		for i := m.branchOffset; i < end; i++ {
			br := m.branches[i]

			cursor := "  "
			if i == m.branchCursor {
				cursor = styleCursor.Render("▶ ")
			}

			name := fmt.Sprintf("%-*s", nameWidth, br)

			current := ""
			if br == m.gitBranch {
				current = "  " + styleDim.Render("(current)")
			}

			fmt.Fprintf(&b, "%s%s%s\n", cursor, name, current)
		}
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("↑↓/jk navigate · enter switch · esc cancel"))

	return b.String()
}

func (m model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(m.renderBrandHeader())
	b.WriteString("\n")
	for _, line := range m.confirmLines {
		b.WriteString(styleCyan.Render(line) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styleBold.Render("Apply? [y/n]"))
	return b.String()
}

func (m model) viewResult() string {
	var b strings.Builder
	if m.applyErr != nil {
		b.WriteString(styleRed.Render("Error:") + " " + m.applyErr.Error() + "\n")
	} else {
		b.WriteString(styleGreen.Render("Done.") + "\n")
	}
	for _, line := range m.applyLog {
		b.WriteString("  " + line + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("Press any key to return."))
	return b.String()
}
