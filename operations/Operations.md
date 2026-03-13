# Operations

Central hub for tracking features, tasks, and project progress in a structured way.

## Structure

- Each feature gets its own folder: `feature-<name>/`
- Each folder contains a main `README.md` tracking: goal, scope, status, and tasks
- Additional docs, assets, or notes live alongside in the same folder
- Status values: `planned` | `in-progress` | `done` | `blocked`

## Active Features

| Feature | Status | File |
|---------|--------|------|
| CWD File Explorer | in-progress | [feature-cwd/README.md](feature-cwd/README.md) |

## Bug Fixes

| Bug | Status | File |
|-----|--------|------|
| Empty filter in mcp_tools.go | done | [bugfix-empty-filter/README.md](bugfix-empty-filter/README.md) |

## Process

1. **Plan** — Define the feature scope and break it into tasks
2. **Build** — Work through tasks, updating status as we go
3. **Blockage** — Document blockers, dependencies, or issues stalling progress
4. **Review** — Verify everything works, close out the feature
