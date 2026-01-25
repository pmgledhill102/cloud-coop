# Test Data

This directory contains test fixtures and data files used by tests throughout the project.

## Directory Structure

```
testdata/
  fixtures/           # JSON, YAML, and other data files for tests
  golden/             # Expected output files for golden tests
  configs/            # Sample configuration files for testing
```

## Usage

Go's testing framework automatically makes files in `testdata/` directories
available during testing. Access them using relative paths:

```go
func TestWithFixture(t *testing.T) {
    data, err := os.ReadFile("testdata/fixtures/sample.json")
    if err != nil {
        t.Fatal(err)
    }
    // Use data in test...
}
```

## Conventions

1. **Naming**: Use descriptive names that indicate what the file tests
   - `valid_config.yaml` - A correctly formatted config
   - `missing_required_field.yaml` - Config missing a required field
   - `unicode_names.json` - Data with unicode characters

2. **Organization**: Group related fixtures in subdirectories

3. **Size**: Keep fixtures small and focused on specific test scenarios

4. **Golden Files**: For output comparison tests, store expected output in `golden/`
   and use the `-update` flag pattern to regenerate them
