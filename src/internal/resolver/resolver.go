package resolver

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leberkas-org/maggus/internal/gitutil"

	"github.com/leberkas-org/maggus/internal/globalconfig"
)

// PromptFunc asks the user a yes/no question and returns true for yes.
type PromptFunc func(question string) bool

// Deps holds injectable dependencies for testing.
type Deps struct {
	// LoadConfig loads the global config. Defaults to globalconfig.Load.
	LoadConfig func() (globalconfig.GlobalConfig, error)
	// SaveConfig saves the global config. Defaults to globalconfig.Save.
	SaveConfig func(globalconfig.GlobalConfig) error
	// IsGitRepo checks whether the given directory is inside a git repository.
	IsGitRepo func(dir string) bool
	// Prompt asks the user a yes/no question.
	Prompt PromptFunc
	// Chdir changes the working directory.
	Chdir func(dir string) error
	// DirExists checks whether a directory exists.
	DirExists func(dir string) bool
}

// DefaultDeps returns Deps wired to real implementations.
func DefaultDeps() Deps {
	return Deps{
		LoadConfig: globalconfig.Load,
		SaveConfig: globalconfig.Save,
		IsGitRepo:  isGitRepo,
		Prompt:     nil, // must be provided by caller
		Chdir:      os.Chdir,
		DirExists:  dirExists,
	}
}

// Result describes what the resolver decided.
type Result struct {
	// Dir is the resolved working directory (absolute path).
	Dir string
	// Changed is true if os.Chdir was called (dir differs from original cwd).
	Changed bool
}

// Resolve determines which repository directory to use on startup.
//
// Resolution order:
//  1. If cwd matches a configured repository, use it.
//  2. If cwd is a git repo but not configured, prompt to add it; use it either way.
//  3. If cwd is not a git repo, switch to last_opened.
//  4. If last_opened doesn't exist, fall back to first available configured repo.
//  5. If no repos are configured, stay in cwd.
//
// When a repository is selected, last_opened is updated in the global config.
func Resolve(cwd string, deps Deps) (Result, error) {
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return Result{}, fmt.Errorf("resolve absolute path: %w", err)
	}

	cfg, err := deps.LoadConfig()
	if err != nil {
		return Result{}, fmt.Errorf("load global config: %w", err)
	}

	// Case 1: cwd is a configured repository.
	if cfg.HasRepository(absCwd) {
		return selectDir(absCwd, absCwd, cfg, deps)
	}

	// Case 2: cwd is a git repo but not configured.
	if deps.IsGitRepo(absCwd) {
		if deps.Prompt != nil && deps.Prompt(fmt.Sprintf("Current directory %s is a git repo but not in your repository list. Add it?", absCwd)) {
			cfg.AddRepository(absCwd)
		}
		return selectDir(absCwd, absCwd, cfg, deps)
	}

	// Case 3: cwd is neither configured nor a git repo — try last_opened.
	if cfg.LastOpened != "" && deps.DirExists(cfg.LastOpened) {
		return selectDir(cfg.LastOpened, absCwd, cfg, deps)
	}

	// Case 4: last_opened missing or invalid — try first available configured repo.
	for _, repo := range cfg.Repositories {
		if deps.DirExists(repo.Path) {
			return selectDir(repo.Path, absCwd, cfg, deps)
		}
	}

	// Case 5: no repos configured — stay in cwd.
	return Result{Dir: absCwd, Changed: false}, nil
}

// selectDir switches to the target directory (if different from cwd),
// updates last_opened, and saves the config.
func selectDir(target, cwd string, cfg globalconfig.GlobalConfig, deps Deps) (Result, error) {
	changed := target != cwd
	if changed {
		if err := deps.Chdir(target); err != nil {
			return Result{}, fmt.Errorf("chdir to %s: %w", target, err)
		}
	}

	cfg.SetLastOpened(target)
	if err := deps.SaveConfig(cfg); err != nil {
		// Non-fatal: log but don't fail startup.
		fmt.Fprintf(os.Stderr, "warning: could not update last_opened: %v\n", err)
	}

	return Result{Dir: target, Changed: changed}, nil
}

// isGitRepo checks if the directory is inside a git work tree.
func isGitRepo(dir string) bool {
	cmd := gitutil.Command("-C", dir, "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// dirExists checks if a directory exists and is a directory.
func dirExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}
