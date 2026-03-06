# Grroxy

### Project Structure

`/apps` main apps (launcher, app, tool runner)  
`/cmd` go binaries  
`/grx` grroxy engine (core packages)  
`/internal` database schemas, database types, save fn, etc.  
`/docs` documentation

### Latest Versions

```bash
# Current Version (see VERSION file)
Backend:   v0.26.0
Frontend:  v0.26.0

# Released App
App:       v2026.3.3
Backend:   v0.26.0
Frontend:  v0.26.0
```

The version is maintained in the `VERSION` file at the project root. Use `internal/version` package to access version programmatically in Go code.
