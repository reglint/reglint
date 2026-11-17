package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCollectRootFilesUsesCandidateSetWithoutWalking(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod permissions are unreliable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores permission bits")
	}

	root := t.TempDir()
	writeFileWithContent(t, filepath.Join(root, "a.php"), "zzz")
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatalf("failed to chmod directory: %v", err)
	}
	defer func() {
		_ = os.Chmod(locked, 0o755)
	}()

	files, skipped, warnings, err := collectRootFiles(
		root,
		[]string{"**/*"},
		nil,
		nil,
		map[string]struct{}{"a.php": {}},
		1024,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 || files[0] != "a.php" {
		t.Fatalf("expected only a.php, got %v", files)
	}
	if skipped != 0 {
		t.Fatalf("expected 0 skipped, got %d", skipped)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings (no tree walk), got %v", warnings)
	}
}

func TestCollectRootFilesUsesConfiguredWalker(t *testing.T) {
	root := t.TempDir()
	called := false

	originalWalker := walkRootTree
	walkRootTree = func(rootPath string, fn fs.WalkDirFunc) error {
		called = true
		info, err := os.Stat(rootPath)
		if err != nil {
			return err
		}

		return fn(rootPath, fs.FileInfoToDirEntry(info), nil)
	}
	t.Cleanup(func() {
		walkRootTree = originalWalker
	})

	files, skipped, _, err := collectRootFiles(root, []string{"**/*"}, nil, nil, nil, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected configured walker to be called")
	}
	if len(files) != 0 {
		t.Fatalf("expected no files, got %v", files)
	}
	if skipped != 0 {
		t.Fatalf("expected no skipped files, got %d", skipped)
	}
}
