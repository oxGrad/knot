package linker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/resolver"
)

// OpType describes the kind of action to take for a symlink.
type OpType int

const (
	OpCreate   OpType = iota // symlink will be created
	OpExists                 // correct symlink already present — no-op
	OpConflict               // target exists but is not the expected symlink
	OpBroken                 // target is a symlink pointing nowhere
	OpRemove                 // symlink will be removed (untie)
	OpSkip                   // ignored by pattern or condition
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
	default:
		return "unknown"
	}
}

// LinkAction describes a single planned filesystem operation.
type LinkAction struct {
	Package string
	Source  string // absolute path to source file
	Target  string // absolute path where symlink will be created
	Op      OpType
	Reason  string
}

// Linker is the core engine for managing symlinks.
type Linker struct {
	DryRun  bool
	HomeDir string
	GOOS    string
	Writer  io.Writer
}

// New creates a Linker with defaults from the current environment.
func New(dryRun bool) *Linker {
	home, _ := os.UserHomeDir()
	return &Linker{
		DryRun:  dryRun,
		HomeDir: home,
		GOOS:    runtime.GOOS,
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

		pkgActions, err := l.planPackage(name, source, target, pkg.Ignore)
		if err != nil {
			return nil, fmt.Errorf("planning package %q: %w", name, err)
		}
		actions = append(actions, pkgActions...)
	}

	return actions, nil
}

// planPackage walks the source directory and computes link actions for each file.
func (l *Linker) planPackage(name, source, target string, ignorePatterns []string) ([]LinkAction, error) {
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("source %q: %w", source, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source %q is not a directory", source)
	}

	var actions []LinkAction

	err = filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil // only symlink files, not directories
		}

		ignored, err := resolver.ShouldIgnore(path, ignorePatterns)
		if err != nil {
			return fmt.Errorf("checking ignore for %q: %w", path, err)
		}
		if ignored {
			actions = append(actions, LinkAction{
				Package: name,
				Source:  path,
				Op:      OpSkip,
				Reason:  "matched ignore pattern",
			})
			return nil
		}

		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		linkTarget := filepath.Join(target, rel)

		op, reason := l.classifyTarget(path, linkTarget)
		actions = append(actions, LinkAction{
			Package: name,
			Source:  path,
			Target:  linkTarget,
			Op:      op,
			Reason:  reason,
		})
		return nil
	})

	return actions, err
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
		if a.Op == OpExists {
			actions[i].Op = OpRemove
			actions[i].Reason = "removing symlink"
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

		case OpExists:
			// no-op, already correctly linked

		case OpConflict:
			_, _ = fmt.Fprintf(l.Writer, "[CONFLICT] %s: %s\n", a.Target, a.Reason)

		case OpBroken:
			_, _ = fmt.Fprintf(l.Writer, "[BROKEN]   %s: %s\n", a.Target, a.Reason)

		case OpSkip:
			// silent skip
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
		case OpCreate:
			_, _ = fmt.Fprintf(l.Writer, "[MISSING]  %s\n", a.Target)
		case OpConflict:
			_, _ = fmt.Fprintf(l.Writer, "[CONFLICT] %s: %s\n", a.Target, a.Reason)
		case OpBroken:
			_, _ = fmt.Fprintf(l.Writer, "[BROKEN]   %s\n", a.Target)
		case OpSkip:
			// silent
		}
	}

	_, _ = fmt.Fprintf(l.Writer, "\n%d ok, %d missing, %d conflict, %d broken\n",
		counts[OpExists], counts[OpCreate], counts[OpConflict], counts[OpBroken])
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
		case OpSkip:
			// silent
		}
	}

	_, _ = fmt.Fprintf(l.Writer, "\nPlan: %d to create, %d to remove, %d already linked, %d conflicts\n",
		counts[OpCreate], counts[OpRemove], counts[OpExists], counts[OpConflict])
}
