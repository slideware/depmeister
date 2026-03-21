# Depmeister

A dependency scanner that checks your project's dependencies against [deps.dev](https://deps.dev) (Google Open Source Insights) and flags:

- **Recently published packages** — catch supply chain attacks by flagging packages newer than a configurable threshold
- **Known vulnerabilities** — flag packages with OSV/advisory data
- **Low OpenSSF Scorecard scores** — flag packages with poor security practices

Supports **Go** (`go.mod`), **NPM** (`package-lock.json` / `package.json`), **Python** (`requirements.txt`), and **Rust** (`Cargo.lock` / `Cargo.toml`).

Designed to run both locally and in CI — exits with code 1 when issues are found.

## Install

```sh
go install github.com/slideware/depmeister/cmd/depmeister@latest
```

## Usage

```sh
# Scan current directory
depmeister

# Scan a specific directory
depmeister --dir /path/to/project

# Recursively scan subdirectories (e.g. monorepos)
depmeister --dir /path/to/monorepo --recursive

# JSON output for CI integration
depmeister --format json

# Debug logging
depmeister --verbose
```

### Flags

| Flag          | Default            | Description                     |
| ------------- | ------------------ | ------------------------------- |
| `--dir`       | `.`                | Directory to scan               |
| `--recursive` | `false`            | Recursively scan subdirectories |
| `--config`    | `.depmeister.yaml` | Path to config file             |
| `--format`    | `text`             | Output format: `text` or `json` |
| `--verbose`   | `false`            | Enable debug logging            |

## Configuration

Create a `.depmeister.yaml` in your project root. All fields are optional, defaults are applied for anything not specified.

```yaml
max_age_days: 7 # Flag packages published less than X days ago (0 = disabled)
min_scorecard: 2.0 # Minimum OpenSSF Scorecard score (0 = disabled)
fail_on_vulns: true # Flag packages with known vulnerabilities
skip_indirect: true # Skip indirect dependencies (go.mod // indirect)
skip_dev_deps: false # Skip devDependencies (npm)
concurrency: 10 # Max concurrent API requests

allowlist:
  - name: "some-internal-pkg"
    reason: "Internal package, not on deps.dev"
    vulns: true # Suppress vulnerability warnings
    age: true # Suppress age warnings
    score: true # Suppress scorecard warnings
```

### Environment variables

All config values can be overridden with environment variables using the `DEPMEISTER_` prefix:

| Variable                   | Example            |
| -------------------------- | ------------------ |
| `DEPMEISTER_MAX_AGE_DAYS`  | `14`               |
| `DEPMEISTER_MIN_SCORECARD` | `7.0`              |
| `DEPMEISTER_FAIL_ON_VULNS` | `false`            |
| `DEPMEISTER_SKIP_INDIRECT` | `true`             |
| `DEPMEISTER_SKIP_DEV_DEPS` | `true`             |
| `DEPMEISTER_CONCURRENCY`   | `20`               |
| `DEPMEISTER_DIR`           | `/path/to/project` |
| `DEPMEISTER_FORMAT`        | `json`             |
| `DEPMEISTER_RECURSIVE`     | `true`             |
| `DEPMEISTER_VERBOSE`       | `true`             |

### Priority order

1. CLI flags (highest)
2. Environment variables
3. YAML config file
4. Built-in defaults (lowest)

## CI examples

### GitHub Actions

```yaml
# .github/workflows/depmeister.yml
name: Dependency Check
on: [push, pull_request]

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - run: go install github.com/slideware/depmeister/cmd/depmeister@latest
      - run: depmeister --format text
```

### GitLab CI

```yaml
# .gitlab-ci.yml
depmeister:
  image: golang:latest
  stage: test
  script:
    - go install github.com/slideware/depmeister/cmd/depmeister@latest
    - depmeister --format text
```

## License

BSD 3-Clause. See [LICENSE](LICENSE).
