package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixtures_MustLoad(t *testing.T) {
	// Create a temporary testdata directory
	tempDir := t.TempDir()
	testdataDir := filepath.Join(tempDir, "testdata")
	if err := os.MkdirAll(testdataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a test fixture file
	testFile := filepath.Join(testdataDir, "sample.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	fixtures := NewFixtures(t, testdataDir)
	data := fixtures.MustLoad("sample.txt")

	if string(data) != "hello world" {
		t.Errorf("unexpected content: %s", string(data))
	}
}

func TestFixtures_MustLoadString(t *testing.T) {
	tempDir := t.TempDir()
	testdataDir := filepath.Join(tempDir, "testdata")
	if err := os.MkdirAll(testdataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(testdataDir, "text.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0o644); err != nil {
		t.Fatal(err)
	}

	fixtures := NewFixtures(t, testdataDir)
	content := fixtures.MustLoadString("text.txt")

	if content != "test content" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestFixtures_MustLoadJSON(t *testing.T) {
	tempDir := t.TempDir()
	testdataDir := filepath.Join(tempDir, "testdata")
	if err := os.MkdirAll(testdataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(testdataDir, "data.json")
	if err := os.WriteFile(testFile, []byte(`{"name":"test","value":42}`), 0o644); err != nil {
		t.Fatal(err)
	}

	fixtures := NewFixtures(t, testdataDir)

	var result struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	fixtures.MustLoadJSON("data.json", &result)

	if result.Name != "test" || result.Value != 42 {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestFixtures_Path(t *testing.T) {
	fixtures := NewFixtures(t, "/some/base/dir")
	path := fixtures.Path("subdir/file.txt")

	expected := "/some/base/dir/subdir/file.txt"
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestFixtures_Exists(t *testing.T) {
	tempDir := t.TempDir()
	testdataDir := filepath.Join(tempDir, "testdata")
	if err := os.MkdirAll(testdataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(testdataDir, "exists.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	fixtures := NewFixtures(t, testdataDir)

	if !fixtures.Exists("exists.txt") {
		t.Error("expected exists.txt to exist")
	}
	if fixtures.Exists("nonexistent.txt") {
		t.Error("expected nonexistent.txt to not exist")
	}
}

func TestFixtures_TempFixtureDir(t *testing.T) {
	// Create source testdata
	tempDir := t.TempDir()
	testdataDir := filepath.Join(tempDir, "testdata")
	if err := os.MkdirAll(testdataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(testdataDir, "config.yaml")
	if err := os.WriteFile(testFile, []byte("key: value"), 0o644); err != nil {
		t.Fatal(err)
	}

	fixtures := NewFixtures(t, testdataDir)
	tempFixDir := fixtures.TempFixtureDir("config.yaml")

	// Check that the file was copied
	copiedFile := filepath.Join(tempFixDir, "config.yaml")
	data, err := os.ReadFile(copiedFile)
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if string(data) != "key: value" {
		t.Errorf("unexpected content: %s", string(data))
	}

	// Verify we can modify the copy without affecting original
	if err := os.WriteFile(copiedFile, []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalData, _ := os.ReadFile(testFile)
	if string(originalData) != "key: value" {
		t.Error("original file was modified")
	}
}

func TestFixtures_GoldenFile(t *testing.T) {
	tempDir := t.TempDir()
	testdataDir := filepath.Join(tempDir, "testdata")
	if err := os.MkdirAll(testdataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a golden file
	goldenFile := filepath.Join(testdataDir, "output.golden")
	if err := os.WriteFile(goldenFile, []byte("expected output"), 0o644); err != nil {
		t.Fatal(err)
	}

	fixtures := NewFixtures(t, testdataDir)

	// This should pass - actual matches expected
	fixtures.GoldenFile("output.golden", []byte("expected output"))
}

func TestFixtures_GoldenFile_Update(t *testing.T) {
	tempDir := t.TempDir()
	testdataDir := filepath.Join(tempDir, "testdata")
	if err := os.MkdirAll(testdataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	fixtures := NewFixtures(t, testdataDir)

	// Enable update mode temporarily
	oldUpdate := UpdateGoldenFiles
	UpdateGoldenFiles = true
	defer func() { UpdateGoldenFiles = oldUpdate }()

	// This should create/update the golden file
	fixtures.GoldenFile("new.golden", []byte("new content"))

	// Verify the file was created
	goldenFile := filepath.Join(testdataDir, "new.golden")
	data, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("golden file was not created: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("unexpected golden file content: %s", string(data))
	}
}
