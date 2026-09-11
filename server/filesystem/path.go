package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var ErrPathEscapesRoot = errors.New("path escapes storage root")

// ValidatePath normalizes a path within a storage namespace. A leading slash
// denotes the virtual storage root. Paths that escape that root are rejected.
func ValidatePath(name string) (string, error) {
	if strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("%w: path contains NUL", ErrPathEscapesRoot)
	}

	name = strings.ReplaceAll(name, "\\", "/")
	if len(name) >= 2 && isASCIIAlpha(name[0]) && name[1] == ':' {
		return "", fmt.Errorf("%w: volume-qualified path", ErrPathEscapesRoot)
	}
	name = strings.TrimLeft(name, "/")
	name = path.Clean(name)
	if name == "." {
		return "", nil
	}
	if name == ".." || strings.HasPrefix(name, "../") {
		return "", ErrPathEscapesRoot
	}
	return name, nil
}

// ResolvePath joins a validated storage-relative path to a configured storage
// root. It is used for rclone paths, whose root may name a remote backend.
func ResolvePath(root, name string) (string, error) {
	if root == "" {
		return "", errors.New("storage root cannot be empty")
	}
	relative, err := ValidatePath(name)
	if err != nil {
		return "", err
	}
	if relative == "" {
		return root, nil
	}
	return strings.TrimRight(root, "/") + "/" + relative, nil
}

// ResolveLocalPath resolves a storage path beneath root. In addition to
// lexical traversal, it rejects escapes through any existing symlink ancestor.
func ResolveLocalPath(root, name string) (string, error) {
	if root == "" {
		return "", errors.New("storage root cannot be empty")
	}
	relative, err := ValidatePath(name)
	if err != nil {
		return "", err
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absRoot = filepath.Clean(absRoot)
	rootAncestor, resolvedRootAncestor, err := resolveExistingAncestor(absRoot)
	if err != nil {
		return "", fmt.Errorf("resolve storage root: %w", err)
	}
	missingRootSuffix, err := filepath.Rel(rootAncestor, absRoot)
	if err != nil || escapesRoot(missingRootSuffix) {
		return "", ErrPathEscapesRoot
	}
	resolvedRoot := filepath.Join(resolvedRootAncestor, missingRootSuffix)

	fullPath := filepath.Join(absRoot, filepath.FromSlash(relative))
	pathFromRoot, err := filepath.Rel(absRoot, fullPath)
	if err != nil || escapesRoot(pathFromRoot) {
		return "", ErrPathEscapesRoot
	}

	existingAncestor, resolvedPath, err := resolveExistingAncestor(fullPath)
	if err != nil {
		return "", err
	}
	existingFromRoot, err := filepath.Rel(absRoot, existingAncestor)
	if err != nil {
		return "", err
	}
	// If root does not exist yet, the closest existing component is above it and
	// there cannot be a symlink inside the root to inspect.
	if !escapesRoot(existingFromRoot) {
		resolvedFromRoot, relErr := filepath.Rel(resolvedRoot, resolvedPath)
		if relErr != nil || escapesRoot(resolvedFromRoot) {
			return "", fmt.Errorf("%w through symlink", ErrPathEscapesRoot)
		}
	}

	return fullPath, nil
}

// ValidateLocalTreeDestination verifies that every path in sourceRoot can be
// written beneath prefix in the configured local storage root.
func ValidateLocalTreeDestination(root, prefix, sourceRoot string) error {
	return filepath.WalkDir(sourceRoot, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		if relative == "." {
			relative = ""
		}
		_, err = ResolveLocalPath(root, path.Join(prefix, filepath.ToSlash(relative)))
		return err
	})
}

func resolveExistingAncestor(name string) (string, string, error) {
	ancestor := name
	for {
		resolved, err := filepath.EvalSymlinks(ancestor)
		if err == nil {
			return ancestor, resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", "", err
		}
		ancestor = parent
	}
}

func escapesRoot(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func isASCIIAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}
