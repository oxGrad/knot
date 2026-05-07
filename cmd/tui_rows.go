package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
)

func centerLabel(s string) string {
	pad := statusWidth - len(s)
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func (s pkgStatus) label() string {
	switch s {
	case statusTied:
		return styleGreen.Render(centerLabel("tied"))
	case statusUntied:
		return styleDim.Render(centerLabel("untied"))
	case statusPartial:
		return styleYellow.Render(centerLabel("partial"))
	case statusConflict:
		return styleRed.Render(centerLabel("conflict"))
	case statusSkipped:
		return styleDim.Render(centerLabel("skipped"))
	case statusSourceNotFound:
		return styleYellow.Render(centerLabel("no source"))
	}
	return "unknown"
}

func computeStatus(actions []linker.LinkAction) pkgStatus {
	var tied, untied, conflict, skipped, sourceNotFound int
	for _, a := range actions {
		switch a.Op {
		case linker.OpExists:
			tied++
		case linker.OpCreate:
			untied++
		case linker.OpConflict, linker.OpBroken:
			conflict++
		case linker.OpSkip:
			skipped++
		case linker.OpSourceNotFound:
			sourceNotFound++
		}
	}
	nonSkip := tied + untied + conflict
	if nonSkip == 0 && sourceNotFound > 0 {
		return statusSourceNotFound
	}
	if nonSkip == 0 {
		return statusSkipped
	}
	if conflict > 0 {
		return statusConflict
	}
	if tied > 0 && untied == 0 {
		return statusTied
	}
	if untied > 0 && tied == 0 {
		return statusUntied
	}
	return statusPartial
}

func (m *model) pkgPendingArrow(row pkgRow) string {
	if !m.isPending(row) {
		return ""
	}
	target := "untied"
	if m.toggles[row.name] {
		target = "tied"
	}
	return stylePending.Render(" -> " + target)
}

func (m *model) tagWouldBeWord(tr *tagRow) string {
	var tied, untied int
	for _, pkg := range tr.pkgs {
		if pkg.status == statusSkipped || pkg.status == statusConflict || pkg.status == statusSourceNotFound {
			continue
		}
		if m.toggles[pkg.name] {
			tied++
		} else {
			untied++
		}
	}
	if tied > 0 && untied == 0 {
		return "tied"
	}
	if untied > 0 && tied == 0 {
		return "untied"
	}
	return "partial"
}

func (m *model) tagPendingArrow(tr *tagRow) string {
	for _, pkg := range tr.pkgs {
		if m.isPending(pkg) {
			return stylePending.Render(" -> " + m.tagWouldBeWord(tr))
		}
	}
	return ""
}

func buildRows(cfg *config.Config, lnk *linker.Linker) ([]pkgRow, error) {
	names := make([]string, 0, len(cfg.Packages))
	for name := range cfg.Packages {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := make([]pkgRow, 0, len(names))
	for _, name := range names {
		actions, err := lnk.Plan(cfg, []string{name})
		if err != nil {
			return nil, fmt.Errorf("plan %q: %w", name, err)
		}
		rows = append(rows, pkgRow{
			name:    name,
			status:  computeStatus(actions),
			actions: actions,
		})
	}
	return rows, nil
}

func buildTagRows(cfg *config.Config, allRows []pkgRow) []tagRow {
	byTag := config.PackagesByTag(cfg)
	if len(byTag) == 0 {
		return nil
	}

	tagNames := make([]string, 0, len(byTag))
	for name := range byTag {
		tagNames = append(tagNames, name)
	}
	sort.Strings(tagNames)

	rowsByName := make(map[string]pkgRow, len(allRows))
	for _, r := range allRows {
		rowsByName[r.name] = r
	}

	rows := make([]tagRow, 0, len(tagNames))
	for _, tagName := range tagNames {
		pkgNames := byTag[tagName]
		pkgs := make([]pkgRow, 0, len(pkgNames))
		var allActions []linker.LinkAction
		for _, pname := range pkgNames {
			if r, ok := rowsByName[pname]; ok {
				pkgs = append(pkgs, r)
				allActions = append(allActions, r.actions...)
			}
		}
		rows = append(rows, tagRow{
			name:   tagName,
			status: computeStatus(allActions),
			pkgs:   pkgs,
		})
	}
	return rows
}

func visibleTagItems(rows []tagRow) []tagItem {
	var items []tagItem
	for i := range rows {
		tr := &rows[i]
		items = append(items, tagItem{isTag: true, tag: tr})
		if !tr.collapsed {
			for j := range tr.pkgs {
				items = append(items, tagItem{
					isTag:       false,
					pkg:         &tr.pkgs[j],
					tagName:     tr.name,
					isLastChild: j == len(tr.pkgs)-1,
				})
			}
		}
	}
	return items
}

func seedToggles(rows []pkgRow) map[string]bool {
	t := make(map[string]bool, len(rows))
	for _, r := range rows {
		t[r.name] = r.status == statusTied || r.status == statusPartial
	}
	return t
}

func (m *model) isPending(row pkgRow) bool {
	wantTied := m.toggles[row.name]
	currentlyTied := row.status == statusTied || row.status == statusPartial
	return wantTied != currentlyTied
}

func (m *model) pendingCount() int {
	n := 0
	for _, r := range m.rows {
		if m.isPending(r) {
			n++
		}
	}
	return n
}

func (m *model) togglePackage(i int) {
	row := m.rows[i]
	if row.status == statusSkipped || row.status == statusConflict || row.status == statusSourceNotFound {
		return
	}
	m.toggles[row.name] = !m.toggles[row.name]
}

func (m *model) toggleTag(tr *tagRow) {
	eligible := func(pkg pkgRow) bool {
		return pkg.status != statusSkipped && pkg.status != statusConflict
	}
	switch tr.status {
	case statusTied:
		allPending := true
		for _, pkg := range tr.pkgs {
			if !eligible(pkg) {
				continue
			}
			if m.toggles[pkg.name] {
				allPending = false
				break
			}
		}
		want := false
		if allPending {
			want = true
		}
		for _, pkg := range tr.pkgs {
			if !eligible(pkg) {
				continue
			}
			m.toggles[pkg.name] = want
		}
	case statusUntied:
		allPending := true
		for _, pkg := range tr.pkgs {
			if !eligible(pkg) {
				continue
			}
			if !m.toggles[pkg.name] {
				allPending = false
				break
			}
		}
		want := true
		if allPending {
			want = false
		}
		for _, pkg := range tr.pkgs {
			if !eligible(pkg) {
				continue
			}
			m.toggles[pkg.name] = want
		}
	case statusPartial:
		allPending := true
		for _, pkg := range tr.pkgs {
			if !eligible(pkg) {
				continue
			}
			currentlyTied := pkg.status == statusTied || pkg.status == statusPartial
			if !currentlyTied && !m.toggles[pkg.name] {
				allPending = false
				break
			}
		}
		for _, pkg := range tr.pkgs {
			if !eligible(pkg) {
				continue
			}
			currentlyTied := pkg.status == statusTied || pkg.status == statusPartial
			if !currentlyTied {
				if allPending {
					m.toggles[pkg.name] = false
				} else {
					m.toggles[pkg.name] = true
				}
			}
		}
	}
}
