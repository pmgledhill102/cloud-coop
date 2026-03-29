package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Fixtures provides utilities for loading test fixture data from files.
// Fixtures are loaded from a testdata/ directory relative to the test file.
type Fixtures struct {
	t       testing.TB
	baseDir string
}

// NewFixtures creates a new Fixtures loader.
// The baseDir should be the path to the testdata directory.
//
// Example:
//
//	fixtures := testutil.NewFixtures(t, "testdata")
//	data := fixtures.MustLoad("input.json")
func NewFixtures(t testing.TB, baseDir string) *Fixtures {
	t.Helper()
	return &Fixtures{
		t:       t,
		baseDir: baseDir,
	}
}

// NewFixturesFromCaller creates a Fixtures loader using the calling test's
// directory to locate the testdata folder.
//
// Example:
//
//	// If test is in /project/internal/pkg/thing_test.go
//	// this will look in /project/internal/pkg/testdata/
//	fixtures := testutil.NewFixturesFromCaller(t)
func NewFixturesFromCaller(t testing.TB) *Fixtures {
	t.Helper()

	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("failed to get caller information")
	}

	dir := filepath.Dir(filename)
	testdataDir := filepath.Join(dir, "testdata")

	return &Fixtures{
		t:       t,
		baseDir: testdataDir,
	}
}

// Load reads a fixture file and returns its contents.
// Returns the data and any error encountered.
func (f *Fixtures) Load(name string) ([]byte, error) {
	f.t.Helper()

	path := filepath.Join(f.baseDir, name)
	return os.ReadFile(path)
}

// MustLoad reads a fixture file and returns its contents.
// Fails the test if the file cannot be read.
func (f *Fixtures) MustLoad(name string) []byte {
	f.t.Helper()

	data, err := f.Load(name)
	if err != nil {
		f.t.Fatalf("failed to load fixture %s: %v", name, err)
	}
	return data
}

// LoadString reads a fixture file and returns its contents as a string.
func (f *Fixtures) LoadString(name string) (string, error) {
	f.t.Helper()

	data, err := f.Load(name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// MustLoadString reads a fixture file and returns its contents as a string.
// Fails the test if the file cannot be read.
func (f *Fixtures) MustLoadString(name string) string {
	f.t.Helper()

	data := f.MustLoad(name)
	return string(data)
}

// LoadJSON reads a fixture file and unmarshals it into the provided value.
func (f *Fixtures) LoadJSON(name string, v interface{}) error {
	f.t.Helper()

	data, err := f.Load(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// MustLoadJSON reads a fixture file and unmarshals it into the provided value.
// Fails the test if the file cannot be read or parsed.
func (f *Fixtures) MustLoadJSON(name string, v interface{}) {
	f.t.Helper()

	if err := f.LoadJSON(name, v); err != nil {
		f.t.Fatalf("failed to load JSON fixture %s: %v", name, err)
	}
}

// Path returns the full path to a fixture file.
// Useful when you need to pass a file path to a function being tested.
func (f *Fixtures) Path(name string) string {
	return filepath.Join(f.baseDir, name)
}

// Exists checks if a fixture file exists.
func (f *Fixtures) Exists(name string) bool {
	path := filepath.Join(f.baseDir, name)
	_, err := os.Stat(path)
	return err == nil
}

// GoldenFile compares the actual output with a golden file.
// If the -update flag is set (via UpdateGolden), updates the golden file.
// Otherwise, compares actual with the golden file content.
//
// Example:
//
//	output := myFunction()
//	fixtures.GoldenFile("expected_output.golden", output)
func (f *Fixtures) GoldenFile(name string, actual []byte) {
	f.t.Helper()

	path := filepath.Join(f.baseDir, name)

	if UpdateGoldenFiles {
		if err := os.MkdirAll(f.baseDir, 0o750); err != nil {
			f.t.Fatalf("failed to create testdata directory: %v", err)
		}
		if err := os.WriteFile(path, actual, 0o600); err != nil {
			f.t.Fatalf("failed to update golden file %s: %v", name, err)
		}
		return
	}

	expected, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatalf("failed to read golden file %s: %v (run with -update to create)", name, err)
	}

	if string(expected) != string(actual) {
		f.t.Errorf("golden file mismatch for %s:\n--- expected ---\n%s\n--- actual ---\n%s",
			name, string(expected), string(actual))
	}
}

// GoldenString is a convenience wrapper around GoldenFile for string data.
func (f *Fixtures) GoldenString(name string, actual string) {
	f.t.Helper()
	f.GoldenFile(name, []byte(actual))
}

// UpdateGoldenFiles controls whether golden files should be updated.
// Set this in TestMain or via a -update flag.
var UpdateGoldenFiles = false

// TempFixtureDir creates a temporary directory with copies of fixture files.
// Useful for tests that need to modify fixture files.
// Returns the temp directory path; cleanup is automatic via t.Cleanup.
func (f *Fixtures) TempFixtureDir(names ...string) string {
	f.t.Helper()

	tempDir, err := os.MkdirTemp("", "testutil-fixtures-*")
	if err != nil {
		f.t.Fatalf("failed to create temp directory: %v", err)
	}

	f.t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	for _, name := range names {
		src := filepath.Join(f.baseDir, name)
		dst := filepath.Clean(filepath.Join(tempDir, name))

		// Create parent directories if needed
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			f.t.Fatalf("failed to create directory for %s: %v", name, err)
		}

		data, err := os.ReadFile(src)
		if err != nil {
			f.t.Fatalf("failed to read fixture %s: %v", name, err)
		}

		if err := os.WriteFile(dst, data, 0o600); err != nil { //nolint:gosec // G703: dst is filepath.Join(tempDir, name) in test fixtures — no user input
			f.t.Fatalf("failed to write fixture copy %s: %v", name, err)
		}
	}

	return tempDir
}
