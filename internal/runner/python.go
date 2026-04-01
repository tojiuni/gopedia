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
	py := resolvePython(root)
	return &Runner{RepoRoot: root, Python: py}, nil
}

// resolvePython returns the Python binary to use, in priority order:
//  1. GOPEDIA_PYTHON env var (explicit override)
//  2. <repo_root>/.venv/bin/python3 (local venv auto-detect)
//  3. <repo_root>/.venv/bin/python  (local venv fallback name)
//  4. "python3" (system fallback)
func resolvePython(repoRoot string) string {
	if v := strings.TrimSpace(os.Getenv("GOPEDIA_PYTHON")); v != "" {
		return v
	}
	for _, rel := range []string{".venv/bin/python3", ".venv/bin/python"} {
		candidate := filepath.Join(repoRoot, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "python3"
}

// ValidatePython checks that the configured Python binary exists and has grpc importable.
// Returns a non-nil error with a human-readable message if the check fails.
func (r *Runner) ValidatePython() error {
	if _, err := exec.LookPath(r.Python); err != nil {
		if !filepath.IsAbs(r.Python) {
			return fmt.Errorf("python binary %q not found in PATH; set GOPEDIA_PYTHON to the venv python (e.g. .venv/bin/python3): %w", r.Python, err)
		}
		if _, statErr := os.Stat(r.Python); statErr != nil {
			return fmt.Errorf("python binary %q does not exist; set GOPEDIA_PYTHON to the venv python (e.g. .venv/bin/python3): %w", r.Python, statErr)
		}
	}
	cmd := exec.Command(r.Python, "-c", "import grpc")
	cmd.Dir = r.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"python binary %q cannot import grpc (venv packages missing); set GOPEDIA_PYTHON to the venv python (e.g. .venv/bin/python3)\ndetail: %s",
			r.Python, strings.TrimSpace(string(out)),
		)
	}
	return nil
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
