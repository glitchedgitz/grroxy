# Repository Rename: `grroxy-db` -> `grroxy`

**Date:** 2026-03-06
**Status:** Planned
**Old:** `github.com/glitchedgitz/grroxy-db`
**New:** `github.com/glitchedgitz/grroxy`

---

## Migration Order

### Step 1: Update codebase (BEFORE renaming the repo)

Do all code changes while the repo is still named `grroxy-db` so everything builds and tests pass.

#### 1a. Update Go module path

- **`go.mod`** — Change `module github.com/glitchedgitz/grroxy-db` to `module github.com/glitchedgitz/grroxy`

#### 1b. Update all Go imports (106 files)

Every `.go` file importing `github.com/glitchedgitz/grroxy-db/...` must change to `github.com/glitchedgitz/grroxy/...`.

**Affected packages (non-exhaustive):**
- `apps/app/` (17+ files)
- `apps/launcher/` (10+ files)
- `apps/tools/` (5+ files)
- `cmd/grroxy/`, `cmd/grroxy-app/`, `cmd/grroxy-tool/`, `cmd/grroxy-chrome/`, `cmd/grroxy-search/`, `cmd/grx-fuzzer/`, `cmd/grxp/`, `cmd/test/`
- `grx/fuzzer/`, `grx/rawhttp/`, `grx/templates/`
- `internal/config/`, `internal/process/`, `internal/save/`, `internal/sdk/`, `internal/types/`
- `examples/`

**Quick fix:** `find . -name '*.go' -exec sed -i '' 's|github.com/glitchedgitz/grroxy-db|github.com/glitchedgitz/grroxy|g' {} +`

#### 1c. Update hardcoded GitHub URLs

| File | What to change |
|------|---------------|
| `internal/updater/updater.go:15` | `releasesURL` — GitHub API releases URL |
| `apps/launcher/update.go` | Any GitHub repo references |
| `docs/process_management.md` | Documentation references |
| `docs/rawproxy.md` | Documentation references |
| `cmd/grx-fuzzer/README.md` | Documentation references |
| `cmd/grroxy/README.md` | Documentation references |
| `README.md` | Root readme references |

#### 1d. Verify build & tests

```bash
go mod tidy
go build ./...
go test ./...
```

#### 1e. Commit all changes

```bash
git add -A
git commit -m "rename: update module path from grroxy-db to grroxy"
git push origin develop
```

---

### Step 2: Rename the GitHub repository

1. Go to **GitHub > Settings > General > Repository name**
2. Change `grroxy-db` to `grroxy`
3. GitHub will automatically set up a redirect from the old URL

---

### Step 3: Post-rename tasks

#### 3a. Update local git remote

```bash
git remote set-url origin git@github.com:glitchedgitz/grroxy.git
```

#### 3b. Update local directory (optional but recommended)

```bash
cd ..
mv grroxy-db grroxy
```

Or re-clone:
```bash
git clone git@github.com:glitchedgitz/grroxy.git
```

#### 3c. Update Go module proxy cache

```bash
GOPROXY=proxy.golang.org go list -m github.com/glitchedgitz/grroxy@latest
```

#### 3d. Update external references

- Any CI/CD pipelines (GitHub Actions, etc.)
- Electron app config (`cmd/electron/`) if it references the repo
- Any external documentation, wikis, or links
- `install.sh` and `release.sh` — currently don't reference the repo name directly (they're fine)
- Homebrew formulae, package managers, or install scripts hosted elsewhere
- Notify users/collaborators of the new URL

---

## Impact Summary

| Area | Files affected | Risk |
|------|---------------|------|
| Go module path (`go.mod`) | 1 | **High** — breaks all imports if not done |
| Go import statements | ~106 | **High** — mechanical but must be complete |
| GitHub API URL (updater) | 1 | **High** — self-update breaks if missed |
| Documentation | ~5 | Low — cosmetic |
| Git remote | 1 (local config) | Low — GitHub redirects old URL |
| Local directory path | N/A | Low — optional rename |

## Rollback

If something goes wrong after the GitHub rename:
- GitHub repo can be renamed back in Settings
- Git redirects work both ways
- Go module path change can be reverted with another commit
