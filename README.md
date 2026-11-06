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

### JSON output

`--format json` produces a report with three top-level fields:

- `checked` — number of dependencies successfully looked up on deps.dev
- `issues` — one entry per flag raised (a dependency can appear more than once)
- `dependencies` — every dependency found, in scan order, each with a `status` and its own list of issues

```json
{
  "checked": 2,
  "issues": [
    {
      "name": "golang.org/x/sys",
      "version": "v0.42.0",
      "system": "GO",
      "source": "go.mod",
      "reason": "vulnerability: GO-2026-5024",
      "advisoryUrl": "https://deps.dev/advisory/osv/GO-2026-5024"
    }
  ],
  "dependencies": [
    {
      "name": "gopkg.in/yaml.v3",
      "version": "v3.0.1",
      "system": "GO",
      "source": "go.mod",
      "status": "ok",
      "issues": []
    },
    {
      "name": "golang.org/x/sys",
      "version": "v0.42.0",
      "system": "GO",
      "source": "go.mod",
      "indirect": true,
      "status": "flagged",
      "issues": [
        {
          "reason": "vulnerability: GO-2026-5024",
          "advisoryUrl": "https://deps.dev/advisory/osv/GO-2026-5024"
        }
      ]
    }
  ]
}
```

`status` is one of:

| Status      | Meaning                                                          |
| ----------- | ---------------------------------------------------------------- |
| `ok`        | Checked, no issues found                                         |
| `flagged`   | Checked, one or more issues found (listed in `issues`)           |
| `unchecked` | The deps.dev lookup failed, so no checks were run for this entry |

`advisoryUrl` is only present on vulnerability issues. `indirect` and `dev` are only present when `true`; see [Indirect dependencies](#indirect-dependencies) for how `indirect` is determined.

For example, to list every dependency with its status:

```sh
depmeister --format json | jq -r '.dependencies[] | "\(.status)\t\(.name)@\(.version)"'
```

## Configuration

Create a `.depmeister.yaml` in your project root. All fields are optional, defaults are applied for anything not specified.

```yaml
max_age_days: 7 # Flag packages published less than X days ago (0 = disabled)
min_scorecard: 2.0 # Minimum OpenSSF Scorecard score (0 = disabled)
fail_on_vulns: true # Flag packages with known vulnerabilities
skip_indirect: true # Skip indirect dependencies (see "Indirect dependencies" below)
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

### Indirect dependencies

A dependency is _indirect_ when the project uses it only through another dependency. `skip_indirect` (default `true`) leaves these out of the scan, and the JSON report marks them with `"indirect": true`.

How indirect dependencies are identified depends on the source file:

- **`go.mod`**: marked `// indirect`.
- **`package-lock.json` (lockfileVersion 2/3)**: not declared in the root package's `dependencies`, `devDependencies`, `optionalDependencies` or `peerDependencies` as recorded in the lockfile, nor in those of the workspace package it is installed for.
- **`package-lock.json` (lockfileVersion 1)**: not declared in those same four fields of the `package.json` next to the lockfile.
- **`package.json` without a lockfile**: never, since only declared dependencies are listed.
- **`requirements.txt`, `Cargo.lock`**: never, since these files do not distinguish direct from indirect.

For npm, a package hoisted to the top-level `node_modules` is still indirect unless it is declared. A dependency declared by a workspace package counts as direct, whether it is installed in the workspace's own `node_modules` or hoisted to the root. Packages nested inside another package's `node_modules` are not scanned. Workspace packages and links to them are the project's own packages and are not reported.

Lockfile v1 does not record which dependencies were declared. If there is no `package.json` next to a v1 lockfile, every package in it is reported as direct (a debug message is logged).

> **Behavior change in v0.3.0:** earlier versions never marked npm dependencies as indirect, so every package in `package-lock.json` was checked. Since `skip_indirect` defaults to `true`, the default configuration now checks only the dependencies declared in `package.json` and skips transitive npm packages. Set `skip_indirect: false` (or `DEPMEISTER_SKIP_INDIRECT=false`) to keep checking every package in the lockfile.

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
