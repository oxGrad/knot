package linker

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/resolver"
	tmplpkg "github.com/oxgrad/knot/internal/template"
)

// OpType describes the kind of action to take for a symlink.
type OpType int

const (
	OpCreate          OpType = iota // symlink will be created
	OpExists                        // correct symlink already present — no-op
	OpConflict                      // target exists but is not the expected symlink
	OpBroken                        // target is a symlink pointing nowhere
	OpRemove                        // symlink will be removed (untie)
	OpSkip                          // ignored by pattern or condition
	OpSourceNotFound                // source directory does not exist on this machine
	OpRender                        // .tmpl file will be rendered to a real file
	OpRenderExists                  // rendered output matches target content — no-op
	OpRemoveRendered                // rendered file will be deleted (untie)
)

func (o OpType) String() string {
	switch o {
	case OpCreate:
		return "create"
	case OpExists:
		return "exists"
	case OpConflict:
		return "conflict"
	case OpBroken:
		return "broken"
	case OpRemove:
		return "remove"
	case OpSkip:
		return "skip"
	case OpSourceNotFound:
		return "source-not-found"
	case OpRender:
		return "render"
	case OpRenderExists:
		return "render-exists"
	case OpRemoveRendered:
		return "remove-rendered"
	default:
		return "unknown"
	}
}

// LinkAction describes a single planned filesystem operation.
type LinkAction struct {
	Package  string
	Source   string // absolute path to source file
	Target   string // absolute path where symlink or rendered file will be created
	Op       OpType
	Reason   string
	Rendered []byte // non-nil for OpRender; holds pre-rendered template content
}

// Linker is the core engine for managing symlinks.
type Linker struct {
	DryRun  bool
	HomeDir string
	GOOS    string
	GoArch  string
	Writer  io.Writer
}

// New creates a Linker with defaults from the current environment.
func New(dryRun bool) *Linker {
	home, _ := os.UserHomeDir()
	return &Linker{
		DryRun:  dryRun,
		HomeDir: home,
		GOOS:    runtime.GOOS,
		GoArch:  runtime.GOARCH,
		Writer:  os.Stdout,
	}
}

// Plan computes what actions would be taken for the given package names.
// It never modifies the filesystem.
func (l *Linker) Plan(cfg *config.Config, packageNames []string) ([]LinkAction, error) {
	var actions []LinkAction

	for _, name := range packageNames {
		pkg, ok := cfg.Packages[name]
		if !ok {
			return nil, fmt.Errorf("unknown package %q", name)
		}

		if !resolver.EvaluateCondition(pkg.Condition, l.GOOS) {
			actions = append(actions, LinkAction{
				Package: name,
				Op:      OpSkip,
				Reason:  fmt.Sprintf("condition not met (os: %s)", pkg.Condition.OS),
			})
			continue
		}

		source := resolver.ExpandPath(pkg.Source, l.HomeDir)
		target := resolver.ExpandPath(pkg.Target, l.HomeDir)
		perFile := strings.HasSuffix(pkg.Target, "/")

		pkgActions, err := l.planPackage(name, source, target, perFile, pkg.Ignore)
		if err != nil {
			return nil, fmt.Errorf("planning package %q: %w", name, err)
		}
		actions = append(actions, pkgActions...)
	}

	return actions, nil
}

// planPackage computes link action(s) for a package.
// When perFile is true (target ends with "/"), each entry in source is linked
// individually into the target directory. Otherwise a single directory-level
// symlink is planned at target.
// Template rendering (.tmpl files) only activates in per-file mode.
func (l *Linker) planPackage(name, source, target string, perFile bool, ignore []string) ([]LinkAction, error) {
	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return []LinkAction{{
				Package: name,
				Source:  source,
				Target:  target,
				Op:      OpSourceNotFound,
				Reason:  fmt.Sprintf("source directory %q does not exist", source),
			}}, nil
		}
		return nil, fmt.Errorf("source %q: %w", source, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source %q is not a directory", source)
	}

	if perFile {
		return l.planPerFile(name, source, target, ignore)
	}

	op, reason := l.classifyTarget(source, target)
	return []LinkAction{{
		Package: name,
		Source:  source,
		Target:  target,
		Op:      op,
		Reason:  reason,
	}}, nil
}

// planPerFile creates individual link actions for each entry in source, linking
// them into target. Entries matching ignore patterns receive OpSkip actions.
// Files with a .tmpl suffix are rendered to real files instead of being symlinked.
func (l *Linker) planPerFile(name, source, target string, ignore []string) ([]LinkAction, error) {
	entries, err := os.ReadDir(source)
	if err != nil {
		return nil, fmt.Errorf("reading source %q: %w", source, err)
	}

	var (
		actions      []LinkAction
		tmplData     tmplpkg.TemplateData
		tmplDataErr  error
		tmplDataOnce bool
	)

	for _, entry := range entries {
		filename := entry.Name()
		fileSrc := filepath.Join(source, filename)

		shouldIgnore, err := resolver.ShouldIgnore(filename, ignore)
		if err != nil {
			return nil, fmt.Errorf("checking ignore for %q: %w", filename, err)
		}
		if shouldIgnore {
			actions = append(actions, LinkAction{
				Package: name,
				Source:  fileSrc,
				Target:  filepath.Join(target, filename),
				Op:      OpSkip,
				Reason:  "ignored by pattern",
			})
			continue
		}

		if strings.HasSuffix(filename, ".tmpl") {
			// Build template data once, lazily, on the first .tmpl file.
			if !tmplDataOnce {
				tmplData, tmplDataErr = tmplpkg.BuildTemplateData(l.GOOS, l.GoArch, l.HomeDir)
				tmplDataOnce = true
			}
			if tmplDataErr != nil {
				return nil, fmt.Errorf("building template data: %w", tmplDataErr)
			}
			fileTgt := filepath.Join(target, strings.TrimSuffix(filename, ".tmpl"))
			action, err := l.planTemplateFile(name, fileSrc, fileTgt, tmplData)
			if err != nil {
				return nil, fmt.Errorf("planning template %q: %w", filename, err)
			}
			actions = append(actions, action)
			continue
		}

		fileTgt := filepath.Join(target, filename)
		op, reason := l.classifyTarget(fileSrc, fileTgt)
		actions = append(actions, LinkAction{
			Package: name,
			Source:  fileSrc,
			Target:  fileTgt,
			Op:      op,
			Reason:  reason,
		})
	}

	return actions, nil
}

// planTemplateFile computes the action for a single .tmpl source file.
// It renders the template to compare against any existing target content.
func (l *Linker) planTemplateFile(name, srcPath, targetPath string, data tmplpkg.TemplateData) (LinkAction, error) {
	rendered, err := tmplpkg.RenderFile(srcPath, data)
	if err != nil {
		return LinkAction{
			Package: name,
			Source:  srcPath,
			Target:  targetPath,
			Op:      OpConflict,
			Reason:  fmt.Sprintf("template render error: %v", err),
		}, nil
	}

	existing, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return LinkAction{
				Package:  name,
				Source:   srcPath,
				Target:   targetPath,
				Op:       OpRender,
				Reason:   "template file",
				Rendered: rendered,
			}, nil
		}
		// Target exists but is unreadable (e.g., permission error) — conflict.
		return LinkAction{
			Package: name,
			Source:  srcPath,
			Target:  targetPath,
			Op:      OpConflict,
			Reason:  fmt.Sprintf("reading target: %v", readErr),
		}, nil
	}

	if bytes.Equal(existing, rendered) {
		return LinkAction{
			Package: name,
			Source:  srcPath,
			Target:  targetPath,
			Op:      OpRenderExists,
			Reason:  "already rendered",
		}, nil
	}

	return LinkAction{
		Package:  name,
		Source:   srcPath,
		Target:   targetPath,
		Op:       OpRender,
		Reason:   "template output changed",
		Rendered: rendered,
	}, nil
}

// classifyTarget determines the OpType for a given (source, target) pair by inspecting
// the current filesystem state at target.
func (l *Linker) classifyTarget(source, target string) (OpType, string) {
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return OpCreate, "target does not exist"
		}
		return OpConflict, fmt.Sprintf("lstat error: %v", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		dest, err := os.Readlink(target)
		if err != nil {
			return OpConflict, fmt.Sprintf("readlink error: %v", err)
		}
		// Normalize both paths for comparison.
		absSource, _ := filepath.EvalSymlinks(source)
		absDest, err := filepath.EvalSymlinks(dest)
		if err != nil {
			// dest points to a nonexistent path — broken symlink.
			return OpBroken, fmt.Sprintf("symlink points to nonexistent path %q", dest)
		}
		if absDest == absSource || dest == source {
			return OpExists, "already correctly linked"
		}
		return OpConflict, fmt.Sprintf("symlink points to %q instead of %q", dest, source)
	}

	return OpConflict, "target exists and is not a symlink"
}

// PlanUntie computes what actions would be taken to remove symlinks for the given packages.
func (l *Linker) PlanUntie(cfg *config.Config, packageNames []string) ([]LinkAction, error) {
	actions, err := l.Plan(cfg, packageNames)
	if err != nil {
		return nil, err
	}

	for i, a := range actions {
		switch a.Op {
		case OpExists:
			actions[i].Op = OpRemove
			actions[i].Reason = "removing symlink"
		case OpCreate:
			actions[i].Op = OpSkip
			actions[i].Reason = "not linked"
		case OpRenderExists:
			actions[i].Op = OpRemoveRendered
			actions[i].Reason = "removing rendered file"
			actions[i].Rendered = nil
		case OpRender:
			actions[i].Op = OpSkip
			actions[i].Reason = "not rendered"
			actions[i].Rendered = nil
		}
	}
	return actions, nil
}

// Apply executes the given actions on the filesystem.
// It respects DryRun — when true, actions are printed but not applied.
// Errors are accumulated and returned together.
func (l *Linker) Apply(actions []LinkAction) error {
	var errs []error

	for _, a := range actions {
		switch a.Op {
		case OpCreate:
			if l.DryRun {
				_, _ = fmt.Fprintf(l.Writer, "[dry-run] create %s -> %s\n", a.Target, a.Source)
				continue
			}
			if err := os.MkdirAll(filepath.Dir(a.Target), 0o755); err != nil {
				errs = append(errs, fmt.Errorf("mkdir %q: %w", filepath.Dir(a.Target), err))
				continue
			}
			if err := os.Symlink(a.Source, a.Target); err != nil {
				errs = append(errs, fmt.Errorf("symlink %q -> %q: %w", a.Target, a.Source, err))
				continue
			}
			_, _ = fmt.Fprintf(l.Writer, "linked   %s -> %s\n", a.Target, a.Source)

		case OpRemove:
			if l.DryRun {
				_, _ = fmt.Fprintf(l.Writer, "[dry-run] remove %s\n", a.Target)
				continue
			}
			if err := os.Remove(a.Target); err != nil {
				errs = append(errs, fmt.Errorf("remove %q: %w", a.Target, err))
				continue
			}
			_, _ = fmt.Fprintf(l.Writer, "removed  %s\n", a.Target)

		case OpRender:
			if l.DryRun {
				_, _ = fmt.Fprintf(l.Writer, "[dry-run] render %s from %s\n", a.Target, a.Source)
				continue
			}
			if err := os.MkdirAll(filepath.Dir(a.Target), 0o755); err != nil {
				errs = append(errs, fmt.Errorf("mkdir %q: %w", filepath.Dir(a.Target), err))
				continue
			}
			// If a symlink exists at the target path (e.g., leftover from directory mode),
			// remove it before writing so os.WriteFile writes a real file, not through the link.
			if info, err := os.Lstat(a.Target); err == nil && info.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(a.Target); err != nil {
					errs = append(errs, fmt.Errorf("removing stale symlink %q: %w", a.Target, err))
					continue
				}
			}
			if err := os.WriteFile(a.Target, a.Rendered, 0o644); err != nil {
				errs = append(errs, fmt.Errorf("writing rendered file %q: %w", a.Target, err))
				continue
			}
			_, _ = fmt.Fprintf(l.Writer, "rendered %s\n", a.Target)

		case OpRemoveRendered:
			if l.DryRun {
				_, _ = fmt.Fprintf(l.Writer, "[dry-run] remove (rendered) %s\n", a.Target)
				continue
			}
			if err := os.Remove(a.Target); err != nil {
				errs = append(errs, fmt.Errorf("remove %q: %w", a.Target, err))
				continue
			}
			_, _ = fmt.Fprintf(l.Writer, "removed  %s\n", a.Target)

		case OpExists, OpRenderExists:
			// no-op, already correctly linked or rendered

		case OpConflict:
			_, _ = fmt.Fprintf(l.Writer, "[CONFLICT] %s: %s\n", a.Target, a.Reason)

		case OpBroken:
			_, _ = fmt.Fprintf(l.Writer, "[BROKEN]   %s: %s\n", a.Target, a.Reason)

		case OpSkip:
			// silent skip

		case OpSourceNotFound:
			// silent skip — source directory not present on this machine
		}
	}

	return errors.Join(errs...)
}

// Status prints the current symlink status for all packages.
func (l *Linker) Status(cfg *config.Config) error {
	names := make([]string, 0, len(cfg.Packages))
	for name := range cfg.Packages {
		names = append(names, name)
	}

	actions, err := l.Plan(cfg, names)
	if err != nil {
		return err
	}

	counts := map[OpType]int{}
	for _, a := range actions {
		counts[a.Op]++
		switch a.Op {
		case OpExists:
			_, _ = fmt.Fprintf(l.Writer, "[OK]       %s\n", a.Target)
		case OpRenderExists:
			_, _ = fmt.Fprintf(l.Writer, "[OK]       %s (rendered)\n", a.Target)
		case OpCreate:
			_, _ = fmt.Fprintf(l.Writer, "[MISSING]  %s\n", a.Target)
		case OpRender:
			_, _ = fmt.Fprintf(l.Writer, "[MISSING]  %s (template not rendered)\n", a.Target)
		case OpConflict:
			_, _ = fmt.Fprintf(l.Writer, "[CONFLICT] %s: %s\n", a.Target, a.Reason)
		case OpBroken:
			_, _ = fmt.Fprintf(l.Writer, "[BROKEN]   %s\n", a.Target)
		case OpSkip:
			// silent
		case OpSourceNotFound:
			_, _ = fmt.Fprintf(l.Writer, "[NO SOURCE] %s: %s\n", a.Package, a.Reason)
		}
	}

	_, _ = fmt.Fprintf(l.Writer, "\n%d ok, %d missing, %d conflict, %d broken\n",
		counts[OpExists]+counts[OpRenderExists],
		counts[OpCreate]+counts[OpRender],
		counts[OpConflict], counts[OpBroken])
	return nil
}

// PrintPlan renders a human-friendly summary of planned actions.
func (l *Linker) PrintPlan(actions []LinkAction) {
	counts := map[OpType]int{}
	for _, a := range actions {
		counts[a.Op]++
		switch a.Op {
		case OpCreate:
			_, _ = fmt.Fprintf(l.Writer, "  + %s -> %s\n", a.Target, a.Source)
		case OpRemove:
			_, _ = fmt.Fprintf(l.Writer, "  - %s\n", a.Target)
		case OpExists:
			_, _ = fmt.Fprintf(l.Writer, "  = %s (already linked)\n", a.Target)
		case OpConflict:
			_, _ = fmt.Fprintf(l.Writer, "  ! %s (conflict: %s)\n", a.Target, a.Reason)
		case OpBroken:
			_, _ = fmt.Fprintf(l.Writer, "  ~ %s (broken symlink)\n", a.Target)
		case OpRender:
			_, _ = fmt.Fprintf(l.Writer, "  + %s (render from %s)\n", a.Target, a.Source)
		case OpRenderExists:
			_, _ = fmt.Fprintf(l.Writer, "  = %s (already rendered)\n", a.Target)
		case OpRemoveRendered:
			_, _ = fmt.Fprintf(l.Writer, "  - %s (rendered file)\n", a.Target)
		case OpSkip:
			// silent
		case OpSourceNotFound:
			_, _ = fmt.Fprintf(l.Writer, "  ? %s (source not found)\n", a.Package)
		}
	}

	_, _ = fmt.Fprintf(l.Writer, "\nPlan: %d to create, %d to remove, %d already linked, %d conflicts\n",
		counts[OpCreate]+counts[OpRender],
		counts[OpRemove]+counts[OpRemoveRendered],
		counts[OpExists]+counts[OpRenderExists],
		counts[OpConflict])
}
