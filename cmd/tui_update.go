package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oxgrad/knot/internal/config"
)

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		fetchGitInfoCmd(dotfilesDir(m.cfgPath)),
		headerTickCmd(),
	}
	for _, row := range m.rows {
		if pkg, ok := m.cfg.Packages[row.name]; ok {
			bin := row.name
			if pkg.Install != nil && pkg.Install.Bin != "" {
				bin = pkg.Install.Bin
			}
			cmds = append(cmds, checkVersionCmd(row.name, bin))
		}
	}
	return tea.Batch(cmds...)
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

	case versionCheckMsg:
		if m.versionChecked == nil {
			m.versionChecked = make(map[string]bool)
		}
		if m.versions == nil {
			m.versions = make(map[string]string)
		}
		m.versionChecked[msg.pkgName] = true
		if msg.found {
			m.versions[msg.pkgName] = msg.version
		}
		return m, nil

	case installDoneMsg:
		m.installPkg = ""
		m.installMgrs = nil
		m.installAvail = nil
		m.installCursor = 0
		m.installOffset = 0
		if msg.err != nil {
			m.phase = phaseResult
			m.applyLog = []string{fmt.Sprintf("install %s: %v", msg.pkgName, msg.err)}
			m.applyErr = msg.err
			return m, nil
		}
		m.phase = phaseList
		var cmd tea.Cmd
		if pkg, ok := m.cfg.Packages[msg.pkgName]; ok {
			bin := msg.pkgName
			if pkg.Install != nil && pkg.Install.Bin != "" {
				bin = pkg.Install.Bin
			}
			delete(m.versions, msg.pkgName)
			m.versionChecked[msg.pkgName] = false
			cmd = checkVersionCmd(msg.pkgName, bin)
		}
		return m, cmd

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
		case phaseInstallSelect:
			return m.updateInstallSelect(msg)
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

		// Re-fire version checks after reload (config may have changed).
		var vCmds []tea.Cmd
		for k := range m.versionChecked {
			delete(m.versionChecked, k)
		}
		for k := range m.versions {
			delete(m.versions, k)
		}
		for _, row := range m.rows {
			if pkg, ok := m.cfg.Packages[row.name]; ok {
				bin := row.name
				if pkg.Install != nil && pkg.Install.Bin != "" {
					bin = pkg.Install.Bin
				}
				vCmds = append(vCmds, checkVersionCmd(row.name, bin))
			}
		}
		if len(vCmds) > 0 {
			return m, tea.Batch(vCmds...)
		}
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
	case "p":
		m.phase = phaseGitPull
		return m, gitPullCmd(m.cfgPath)
	case "b":
		return m, fetchBranchesCmd(dotfilesDir(m.cfgPath))
	case "i":
		if m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			pkg := m.cfg.Packages[row.name]
			if pkg.Install != nil {
				mgrs, avail := detectAvailableManagers(pkg.Install)
				if len(mgrs) > 0 {
					m.installPkg = row.name
					m.installMgrs = mgrs
					m.installAvail = avail
					m.installCursor = 0
					m.installOffset = 0
					m.phase = phaseInstallSelect
				}
			}
		}
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
	case "p":
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

func (m model) updateInstallSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.installCursor > 0 {
			m.installCursor--
		}
	case "down", "j":
		if m.installCursor < len(m.installMgrs)-1 {
			m.installCursor++
		}
	case "enter":
		if m.installCursor < len(m.installMgrs) {
			kind := m.installMgrs[m.installCursor]
			pkg := m.cfg.Packages[m.installPkg]
			return m, m.buildInstallSequence(kind, pkg.Install)
		}
	case "esc", "q":
		m.phase = phaseList
		m.installPkg = ""
		m.installMgrs = nil
		m.installAvail = nil
		m.installCursor = 0
		m.installOffset = 0
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// buildInstallSequence installs all deps followed by the package itself, using the given manager.
// All commands are chained as a single sh -c "..." to preserve interactive terminal takeover.
func (m model) buildInstallSequence(kind pkgManagerKind, install *config.Install) tea.Cmd {
	var parts []string

	for _, depName := range install.Deps {
		depPkg, ok := m.cfg.Packages[depName]
		if !ok || depPkg.Install == nil {
			continue
		}
		if c := buildInstallCommand(kind, depPkg.Install); c != nil {
			parts = append(parts, strings.Join(c.Args, " "))
		}
	}

	if c := buildInstallCommand(kind, install); c != nil {
		parts = append(parts, strings.Join(c.Args, " "))
	}

	if len(parts) == 0 {
		return func() tea.Msg { return installDoneMsg{pkgName: m.installPkg} }
	}

	script := strings.Join(parts, " && ")
	shellCmd := exec.Command("sh", "-c", script)
	return installCmd(m.installPkg, shellCmd)
}
