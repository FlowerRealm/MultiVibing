package projects

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStorePersistsRecentProjectsSortedByLastOpened(t *testing.T) {
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "config", "projects.json"))
	firstDir := mkdir(t, root, "first")
	secondDir := mkdir(t, root, "second")

	t1 := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	store.SetNowForTest(func() time.Time { return t1 })
	first, err := store.Open(firstDir)
	if err != nil {
		t.Fatalf("open first project: %v", err)
	}
	store.SetNowForTest(func() time.Time { return t2 })
	second, err := store.Open(secondDir)
	if err != nil {
		t.Fatalf("open second project: %v", err)
	}

	reloaded := NewStore(filepath.Join(root, "config", "projects.json"))
	projects, err := reloaded.List()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("project count = %d, want 2", len(projects))
	}
	if projects[0].ID != second.ID || projects[1].ID != first.ID {
		t.Fatalf("projects sorted = %#v, want second then first", projects)
	}
}

func TestStoreListMissingFileReturnsEmptySlice(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "missing", "projects.json"))

	projects, err := store.List()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if projects == nil {
		t.Fatal("projects = nil, want empty slice")
	}
	if len(projects) != 0 {
		t.Fatalf("project count = %d, want 0", len(projects))
	}
}

func TestStoreDeduplicatesByNormalizedPath(t *testing.T) {
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "projects.json"))
	projectDir := mkdir(t, root, "project")

	t1 := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	store.SetNowForTest(func() time.Time { return t1 })
	first, err := store.Open(projectDir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	store.SetNowForTest(func() time.Time { return t2 })
	second, err := store.Open(filepath.Join(projectDir, "."))
	if err != nil {
		t.Fatalf("reopen project: %v", err)
	}

	projects, err := store.List()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("project count = %d, want 1", len(projects))
	}
	if first.ID != second.ID || projects[0].LastOpenedAt != t2 {
		t.Fatalf("dedupe result = %#v, first = %#v, second = %#v", projects[0], first, second)
	}
}

func TestStoreForgetRemovesProject(t *testing.T) {
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "projects.json"))
	projectDir := mkdir(t, root, "project")

	project, err := store.Open(projectDir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	if err := store.Forget(project.ID); err != nil {
		t.Fatalf("forget project: %v", err)
	}
	projects, err := store.List()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("project count = %d, want 0", len(projects))
	}
}

func TestStoreRejectsInvalidProjectPaths(t *testing.T) {
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "projects.json"))
	filePath := filepath.Join(root, "not-a-directory")
	if err := touch(filePath); err != nil {
		t.Fatalf("touch file: %v", err)
	}

	if _, err := store.Open(""); err == nil {
		t.Fatal("Open empty path returned nil error")
	}
	if _, err := store.Open(filepath.Join(root, "missing")); err == nil {
		t.Fatal("Open missing path returned nil error")
	}
	if _, err := store.Open(filePath); err == nil {
		t.Fatal("Open file path returned nil error")
	}
}

func mkdir(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	return path
}

func touch(path string) error {
	return os.WriteFile(path, []byte("not a directory"), 0o600)
}
