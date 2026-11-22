package kernel

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

type suppliedPath struct {
	clean, resolved string
	info            os.FileInfo
}

// checkSuppliedPath requires the supplied and cleaned spellings to name one file.
// A relative path uses base, or the working directory when base is empty.
func checkSuppliedPath(base, name, recovery string) (suppliedPath, error) {
	if !filepath.IsAbs(name) && base != "" {
		name = base + string(filepath.Separator) + name
	}
	supplied, err := absoluteSuppliedPath(name)
	if err != nil {
		return suppliedPath{}, pathFailure(err, "cannot make supplied path absolute: ", recovery)
	}
	info, err := os.Stat(supplied)
	if err != nil {
		return suppliedPath{}, pathFailure(err, "supplied path is missing or unreadable: ", recovery)
	}
	clean := filepath.Clean(supplied)
	cleanInfo, err := os.Stat(clean)
	if err != nil {
		return suppliedPath{}, pathFailure(err, "cleaned path is missing or unreadable: ", recovery)
	}
	if !os.SameFile(info, cleanInfo) {
		return suppliedPath{}, termsafe.NewFailure("supplied and cleaned paths name two different files", recovery)
	}
	resolved, err := filepath.EvalSymlinks(supplied)
	if err != nil {
		return suppliedPath{}, pathFailure(err, "cannot resolve supplied path: ", recovery)
	}
	return suppliedPath{clean: clean, resolved: resolved, info: info}, nil
}

// repositoryRoots includes the caller's root and every accessible listed worktree.
// Git supplies the worktree roots even when the project root is a subdirectory.
func repositoryRoots(root string) ([]string, error) {
	roots := []string{root}
	data, err := actorGitBytes(root, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	for _, field := range bytes.Split(data, []byte{0}) {
		if !bytes.HasPrefix(field, []byte("worktree ")) {
			continue
		}
		path := strings.TrimPrefix(string(field), "worktree ")
		_, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			continue
		}
		if err != nil {
			return nil, pathFailure(err, fmt.Sprintf("listed worktree %s is unreadable: ", path), "restore access to the listed worktree, or repair its Git worktree registration")
		}
		roots = append(roots, path)
	}
	top, err := actorGitBytes(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	// Git adds one line terminator. Other whitespace can be part of the path.
	return append(roots, strings.TrimSuffix(string(top), "\n")), nil
}

// temporaryRoots permits initialization only when Git fails and no physical
// ancestor has a Git entry. Records always require Git's repository roots.
func temporaryRoots(root string) ([]string, error) {
	roots, gitErr := repositoryRoots(root)
	if gitErr == nil {
		return roots, nil
	}
	var exit *exec.ExitError
	if !errors.As(gitErr, &exit) {
		return nil, gitErr
	}
	for dir := root; ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
			return nil, gitErr
		}
		if dir == filepath.Dir(dir) {
			return []string{root}, nil
		}
	}
}

// pathFailure preserves the most specific recovery and the original cause.
// Raw OS and Git errors are escaped only when they first become a failure.
func pathFailure(err error, prefix, recovery string) error {
	if err == nil {
		return nil
	}
	var failure *termsafe.Failure
	if errors.As(err, &failure) {
		return termsafe.PrefixFailure(err, prefix)
	}
	return termsafe.NewWrappedFailure(err, prefix+err.Error(), recovery)
}
