# Feature: CWD File Explorer

## Status: `in-progress`

## Goal

VSCode-like file/folder explorer. Users can browse, open, and preview any file or folder from the current working directory.

## Scope

### Backend (grroxy)
- [x] `POST /api/cwd/browse` - Browse any directory path, returns file list with metadata (name, path, is_dir, size)
- [x] `POST /api/cwd/readfile` - Read file content for preview (with binary detection, 10MB limit)
- [ ] File watching integration for browsed directories

### Frontend (cybernetic-ui)
- [x] Folder navigation (click to open folders)
- [x] Back/Up navigation buttons
- [x] Breadcrumb navigation
- [x] File preview panel (text files)
- [x] Binary file detection
- [x] File size display
- [x] Integrated terminal with CWD
- [ ] Syntax highlighting in preview (CodeMirror integration)
- [ ] File editing/saving
- [ ] Context menu (rename, delete, new file/folder)
- [ ] Drag and drop

## Files Changed

### Backend
- `apps/app/filewatcher.go` - Added `CWDBrowse()` and `CWDReadFile()` endpoints
- `cmd/grroxy-app/serve.go` - Registered new endpoints

### Frontend
- `src/lib/scripts/backend_app.ts` - Added `cwdBrowse()` and `cwdReadFile()` API methods
- `src/lib/organism/CWD.svelte` - Complete rewrite with folder navigation, file preview, breadcrumbs
- `src/routes/dev/cwd/+page.svelte` - Simplified to use enhanced CWD component
