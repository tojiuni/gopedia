// Package runner locates the Gopedia repo and runs Python modules with correct PYTHONPATH.
package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const envRepoRoot = "GOPEDIA_REPO_ROOT"

// Runner executes Python modules from the repository root.
type Runner struct {
	RepoRoot string
	Python   string
}

// NewRunner resolves repo root (env GOPEDIA_REPO_ROOT, then walk for go.mod from cwd) and python binary.
func NewRunner() (*Runner, error) {
	root, err := ResolveRepoRoot()
	if err != nil {
		return nil, err
	}
	py := os.Getenv("GOPEDIA_PYTHON")
	if strings.TrimSpace(py) == "" {
		py = "python3"
	}
	return &Runner{RepoRoot: root, Python: py}, nil
}

// ResolveRepoRoot returns GOPEDIA_REPO_ROOT or the directory containing go.mod walking upward from cwd.
func ResolveRepoRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv(envRepoRoot)); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err != nil {
			return "", fmt.Errorf("%s=%q: no go.mod found: %w", envRepoRoot, abs, err)
		}
		return abs, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("could not find go.mod; set GOPEDIA_REPO_ROOT to the gopedia repository root")
}

// RunModule runs `python -m module args...` with cwd=RepoRoot and PYTHONPATH=RepoRoot.
func (r *Runner) RunModule(ctx context.Context, module string, args ...string) ([]byte, []byte, error) {
	if r == nil {
		return nil, nil, errors.New("nil runner")
	}
	if module == "" {
		return nil, nil, errors.New("empty module")
	}
	cmd := exec.CommandContext(ctx, r.Python, append([]string{"-m", module}, args...)...)
	cmd.Dir = r.RepoRoot
	env := os.Environ()
	env = append(env, "PYTHONPATH="+r.RepoRoot)
	cmd.Env = env
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return []byte(stdout.String()), []byte(stderr.String()), err
}
