package resolver

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/globalconfig"
)

// testDeps creates Deps with sensible defaults for testing.
func testDeps() Deps {
	return Deps{
		LoadConfig: func() (globalconfig.GlobalConfig, error) {
			return globalconfig.GlobalConfig{}, nil
		},
		SaveConfig: func(cfg globalconfig.GlobalConfig) error {
			return nil
		},
		IsGitRepo: func(dir string) bool { return false },
		Prompt:    func(q string) bool { return false },
		Chdir:     func(dir string) error { return nil },
		DirExists: func(dir string) bool { return true },
	}
}

// absPath returns a platform-appropriate absolute path under a temp dir.
func absPath(t *testing.T, base, name string) string {
	t.Helper()
	return filepath.Join(base, name)
}

func TestResolve_ConfiguredRepo(t *testing.T) {
	base := t.TempDir()
	repoPath := absPath(t, base, "myrepo")

	deps := testDeps()
	deps.LoadConfig = func() (globalconfig.GlobalConfig, error) {
		return globalconfig.GlobalConfig{
			Repositories: []globalconfig.Repository{{Path: repoPath}},
		}, nil
	}

	var savedCfg globalconfig.GlobalConfig
	deps.SaveConfig = func(cfg globalconfig.GlobalConfig) error {
		savedCfg = cfg
		return nil
	}

	result, err := Resolve(repoPath, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Dir != repoPath {
		t.Fatalf("expected %s, got %q", repoPath, result.Dir)
	}
	if result.Changed {
		t.Fatal("expected Changed=false for matching cwd")
	}
	if savedCfg.LastOpened != repoPath {
		t.Fatalf("expected last_opened updated, got %q", savedCfg.LastOpened)
	}
}

func TestResolve_UnconfiguredGitRepo_AddYes(t *testing.T) {
	base := t.TempDir()
	repoPath := absPath(t, base, "newrepo")

	deps := testDeps()
	deps.IsGitRepo = func(dir string) bool { return true }
	deps.Prompt = func(q string) bool { return true }

	var savedCfg globalconfig.GlobalConfig
	deps.SaveConfig = func(cfg globalconfig.GlobalConfig) error {
		savedCfg = cfg
		return nil
	}

	result, err := Resolve(repoPath, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Dir != repoPath {
		t.Fatalf("expected %s, got %q", repoPath, result.Dir)
	}
	if result.Changed {
		t.Fatal("expected Changed=false (staying in cwd)")
	}
	if !savedCfg.HasRepository(repoPath) {
		t.Fatal("expected repo to be added to config")
	}
	if savedCfg.LastOpened != repoPath {
		t.Fatalf("expected last_opened=%s, got %q", repoPath, savedCfg.LastOpened)
	}
}

func TestResolve_UnconfiguredGitRepo_AddNo(t *testing.T) {
	base := t.TempDir()
	repoPath := absPath(t, base, "newrepo")

	deps := testDeps()
	deps.IsGitRepo = func(dir string) bool { return true }
	deps.Prompt = func(q string) bool { return false }

	var savedCfg globalconfig.GlobalConfig
	deps.SaveConfig = func(cfg globalconfig.GlobalConfig) error {
		savedCfg = cfg
		return nil
	}

	result, err := Resolve(repoPath, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Dir != repoPath {
		t.Fatalf("expected %s, got %q", repoPath, result.Dir)
	}
	if savedCfg.HasRepository(repoPath) {
		t.Fatal("expected repo NOT to be added")
	}
	if savedCfg.LastOpened != repoPath {
		t.Fatalf("expected last_opened=%s, got %q", repoPath, savedCfg.LastOpened)
	}
}

func TestResolve_NonGitDir_FallbackLastOpened(t *testing.T) {
	base := t.TempDir()
	cwd := absPath(t, base, "home")
	target := absPath(t, base, "main")

	deps := testDeps()
	deps.LoadConfig = func() (globalconfig.GlobalConfig, error) {
		return globalconfig.GlobalConfig{
			Repositories: []globalconfig.Repository{{Path: target}},
			LastOpened:   target,
		}, nil
	}
	deps.IsGitRepo = func(dir string) bool { return false }

	var chdirTarget string
	deps.Chdir = func(dir string) error {
		chdirTarget = dir
		return nil
	}

	result, err := Resolve(cwd, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Dir != target {
		t.Fatalf("expected %s, got %q", target, result.Dir)
	}
	if !result.Changed {
		t.Fatal("expected Changed=true")
	}
	if chdirTarget != target {
		t.Fatalf("expected chdir to %s, got %q", target, chdirTarget)
	}
}

func TestResolve_LastOpenedMissing_FallbackFirstRepo(t *testing.T) {
	base := t.TempDir()
	cwd := absPath(t, base, "random")
	gone := absPath(t, base, "gone")
	exists := absPath(t, base, "exists")

	deps := testDeps()
	deps.LoadConfig = func() (globalconfig.GlobalConfig, error) {
		return globalconfig.GlobalConfig{
			Repositories: []globalconfig.Repository{
				{Path: gone},
				{Path: exists},
			},
			LastOpened: gone,
		}, nil
	}
	deps.IsGitRepo = func(dir string) bool { return false }
	deps.DirExists = func(dir string) bool { return dir == exists }

	var chdirTarget string
	deps.Chdir = func(dir string) error {
		chdirTarget = dir
		return nil
	}

	result, err := Resolve(cwd, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Dir != exists {
		t.Fatalf("expected %s, got %q", exists, result.Dir)
	}
	if !result.Changed {
		t.Fatal("expected Changed=true")
	}
	if chdirTarget != exists {
		t.Fatalf("expected chdir to %s, got %q", exists, chdirTarget)
	}
}

func TestResolve_NoReposConfigured_StayInCwd(t *testing.T) {
	base := t.TempDir()
	cwd := absPath(t, base, "wherever")

	deps := testDeps()
	deps.IsGitRepo = func(dir string) bool { return false }

	result, err := Resolve(cwd, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Dir != cwd {
		t.Fatalf("expected %s, got %q", cwd, result.Dir)
	}
	if result.Changed {
		t.Fatal("expected Changed=false")
	}
}

func TestResolve_LoadConfigError(t *testing.T) {
	deps := testDeps()
	deps.LoadConfig = func() (globalconfig.GlobalConfig, error) {
		return globalconfig.GlobalConfig{}, errors.New("disk error")
	}

	base := t.TempDir()
	_, err := Resolve(absPath(t, base, "any"), deps)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolve_ChdirError(t *testing.T) {
	base := t.TempDir()
	cwd := absPath(t, base, "other")
	target := absPath(t, base, "target")

	deps := testDeps()
	deps.LoadConfig = func() (globalconfig.GlobalConfig, error) {
		return globalconfig.GlobalConfig{
			LastOpened: target,
		}, nil
	}
	deps.IsGitRepo = func(dir string) bool { return false }
	deps.Chdir = func(dir string) error { return errors.New("permission denied") }

	_, err := Resolve(cwd, deps)
	if err == nil {
		t.Fatal("expected chdir error")
	}
}

func TestResolve_SaveConfigError_NonFatal(t *testing.T) {
	base := t.TempDir()
	repoPath := absPath(t, base, "myrepo")

	deps := testDeps()
	deps.LoadConfig = func() (globalconfig.GlobalConfig, error) {
		return globalconfig.GlobalConfig{
			Repositories: []globalconfig.Repository{{Path: repoPath}},
		}, nil
	}
	deps.SaveConfig = func(cfg globalconfig.GlobalConfig) error {
		return errors.New("write error")
	}

	result, err := Resolve(repoPath, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Dir != repoPath {
		t.Fatalf("expected %s, got %q", repoPath, result.Dir)
	}
}

func TestResolve_AllReposMissing_StayInCwd(t *testing.T) {
	base := t.TempDir()
	cwd := absPath(t, base, "cwd")
	gone1 := absPath(t, base, "gone1")
	gone2 := absPath(t, base, "gone2")

	deps := testDeps()
	deps.LoadConfig = func() (globalconfig.GlobalConfig, error) {
		return globalconfig.GlobalConfig{
			Repositories: []globalconfig.Repository{
				{Path: gone1},
				{Path: gone2},
			},
			LastOpened: gone1,
		}, nil
	}
	deps.IsGitRepo = func(dir string) bool { return false }
	deps.DirExists = func(dir string) bool { return false }

	result, err := Resolve(cwd, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Dir != cwd {
		t.Fatalf("expected %s, got %q", cwd, result.Dir)
	}
	if result.Changed {
		t.Fatal("expected Changed=false")
	}
}

func TestResolve_NilPrompt_SkipsPrompt(t *testing.T) {
	base := t.TempDir()
	repoPath := absPath(t, base, "gitrepo")

	deps := testDeps()
	deps.IsGitRepo = func(dir string) bool { return true }
	deps.Prompt = nil

	var savedCfg globalconfig.GlobalConfig
	deps.SaveConfig = func(cfg globalconfig.GlobalConfig) error {
		savedCfg = cfg
		return nil
	}

	result, err := Resolve(repoPath, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Dir != repoPath {
		t.Fatalf("expected %s, got %q", repoPath, result.Dir)
	}
	if savedCfg.HasRepository(repoPath) {
		t.Fatal("expected repo NOT added when prompt is nil")
	}
}
