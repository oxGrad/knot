package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Init() tea.Cmd {
	return tea.Batch(
		fetchGitInfoCmd(dotfilesDir(m.cfgPath)),
		headerTickCmd(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustOffset()
		m.adjustBranchOffset()
		return m, nil

	case headerTickMsg:
		m.headerFrame++
		return m, headerTickCmd()

	case gitInfoMsg:
		if msg.err == nil {
			m.gitBranch = msg.branch
			m.gitSHA = msg.sha
			m.gitCommitMsg = msg.msg
		}
		return m, nil

	case tea.KeyMsg:
		switch m.phase {
		case phaseList:
			if m.activeTab == tabTags {
				return m.updateTags(msg)
			}
			return m.updateList(msg)
		case phaseConfirm:
			return m.updateConfirm(msg)
		case phaseBranch:
			return m.updateBranch(msg)
		case phaseResult:
			m.phase = phaseList
			m.applyLog = nil
			m.applyErr = nil
			m.toggles = seedToggles(m.rows)
			return m, nil
		case phaseApply, phaseGitPull, phaseCheckout:
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			return m, nil
		}

	case gitPullResultMsg:
		if msg.err != nil {
			m.phase = phaseResult
			m.applyLog = []string{msg.output}
			m.applyErr = fmt.Errorf("git pull: %w", msg.err)
			return m, nil
		}
		dir := dotfilesDir(m.cfgPath)
		return m, tea.Batch(reloadCmd(m.cfgPath, m.lnk), fetchGitInfoCmd(dir))

	case reloadMsg:
		if msg.err != nil {
			m.phase = phaseResult
			m.applyLog = nil
			m.applyErr = msg.err
			return m, nil
		}
		m.cfg = msg.cfg
		m.rows = msg.rows
		m.toggles = seedToggles(m.rows)

		newTagRows := buildTagRows(msg.cfg, msg.rows)
		collapsedState := make(map[string]bool, len(m.tagRows))
		for _, tr := range m.tagRows {
			collapsedState[tr.name] = tr.collapsed
		}
		for i := range newTagRows {
			if c, ok := collapsedState[newTagRows[i].name]; ok {
				newTagRows[i].collapsed = c
			}
		}
		m.tagRows = newTagRows

		m.cursor = min(m.cursor, len(m.rows)-1)
		if m.cursor < 0 {
			m.cursor = 0
		}
		newVisible := visibleTagItems(m.tagRows)
		m.tagCursor = min(m.tagCursor, len(newVisible)-1)
		if m.tagCursor < 0 {
			m.tagCursor = 0
		}
		m.adjustOffset()
		m.adjustTagOffset()
		m.phase = phaseList
		return m, nil

	case applyDoneMsg:
		m.applyLog = msg.log
		m.applyErr = msg.err
		m.phase = phaseResult
		return m, reloadCmd(m.cfgPath, m.lnk)

	case editorDoneMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("editor error: %v", msg.err)
			m.phase = phaseList
			return m, nil
		}
		m.statusMsg = ""
		return m, reloadCmd(m.cfgPath, m.lnk)

	case branchListMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("git branch: %v", msg.err)
			return m, nil
		}
		m.branches = msg.branches
		m.branchCursor = 0
		for i, b := range m.branches {
			if b == m.gitBranch {
				m.branchCursor = i
				break
			}
		}
		m.branchOffset = 0
		m.adjustBranchOffset()
		m.phase = phaseBranch
		return m, nil

	case checkoutDoneMsg:
		if msg.err != nil {
			m.phase = phaseResult
			m.applyLog = []string{strings.TrimSpace(msg.output)}
			m.applyErr = fmt.Errorf("git checkout: %w", msg.err)
			return m, nil
		}
		dir := dotfilesDir(m.cfgPath)
		return m, tea.Batch(reloadCmd(m.cfgPath, m.lnk), fetchGitInfoCmd(dir))
	}

	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.adjustOffset()
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
			m.adjustOffset()
		}
	case " ":
		m.togglePackage(m.cursor)
	case "a":
		if m.pendingCount() == 0 {
			break
		}
		m.confirmLines = m.buildConfirmLines()
		m.phase = phaseConfirm
	case "r":
		m.phase = phaseGitPull
		return m, gitPullCmd(m.cfgPath)
	case "b":
		return m, fetchBranchesCmd(dotfilesDir(m.cfgPath))
	case "e":
		return m, editorCmd(m.cfgPath)
	case "]":
		m.activeTab = tabTags
	case "m":
		m.mascotChar = (m.mascotChar + 1) % 3
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		m.phase = phaseApply
		return m, applyCmd(m.cfg, m.lnk, m.rows, m.toggles)
	case "n", "esc", "q":
		m.phase = phaseList
		m.confirmLines = nil
	}
	return m, nil
}

func (m model) updateBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.branchCursor > 0 {
			m.branchCursor--
			m.adjustBranchOffset()
		}
	case "down", "j":
		if m.branchCursor < len(m.branches)-1 {
			m.branchCursor++
			m.adjustBranchOffset()
		}
	case "enter":
		selected := m.branches[m.branchCursor]
		if selected == m.gitBranch {
			m.phase = phaseList
			break
		}
		m.phase = phaseCheckout
		return m, checkoutBranchCmd(dotfilesDir(m.cfgPath), selected)
	case "esc", "q":
		m.phase = phaseList
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateTags(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := visibleTagItems(m.tagRows)
	switch msg.String() {
	case "up", "k":
		if m.tagCursor > 0 {
			m.tagCursor--
			m.adjustTagOffset()
		}
	case "down", "j":
		if m.tagCursor < len(items)-1 {
			m.tagCursor++
			m.adjustTagOffset()
		}
	case " ":
		if m.tagCursor < len(items) {
			item := items[m.tagCursor]
			if item.isTag {
				m.toggleTag(item.tag)
			} else {
				for i, r := range m.rows {
					if r.name == item.pkg.name {
						m.togglePackage(i)
						break
					}
				}
			}
		}
	case "enter":
		if m.tagCursor < len(items) {
			item := items[m.tagCursor]
			if item.isTag {
				item.tag.collapsed = !item.tag.collapsed
				m.adjustTagOffset()
			}
		}
	case "a":
		if m.pendingCount() == 0 {
			break
		}
		m.confirmLines = m.buildConfirmLines()
		m.phase = phaseConfirm
	case "[":
		m.activeTab = tabPackages
	case "m":
		m.mascotChar = (m.mascotChar + 1) % 3
	case "r":
		m.phase = phaseGitPull
		return m, gitPullCmd(m.cfgPath)
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) buildConfirmLines() []string {
	var lines []string
	for _, row := range m.rows {
		if !m.isPending(row) {
			continue
		}
		if m.toggles[row.name] {
			lines = append(lines, fmt.Sprintf("  tie   %s", row.name))
		} else {
			lines = append(lines, fmt.Sprintf("  untie %s", row.name))
		}
	}
	return lines
}
