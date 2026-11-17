package ignore

import (
	"io/fs"
	"os"
	"testing"
)

func TestIgnoreLoaderWalkUsesConfiguredWalker(t *testing.T) {
	root := t.TempDir()
	called := false

	originalWalker := walkDirectoryTree
	walkDirectoryTree = func(rootPath string, fn fs.WalkDirFunc) error {
		called = true
		info, err := os.Stat(rootPath)
		if err != nil {
			return err
		}

		return fn(rootPath, fs.FileInfoToDirEntry(info), nil)
	}
	t.Cleanup(func() {
		walkDirectoryTree = originalWalker
	})

	loader := newLoader(root, []string{".ignore"})
	if err := loader.walk(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected configured walker to be called")
	}
	if len(loader.directories) != 1 || loader.directories[0] != root {
		t.Fatalf("unexpected directories: %v", loader.directories)
	}
}
