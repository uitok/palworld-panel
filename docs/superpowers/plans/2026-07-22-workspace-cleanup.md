# Workspace Cleanup Implementation Plan

**Goal:** Reduce local disk usage and group local-only reference archives without changing source code, runtime data, or current release packages.

**Approach:** Preserve `dev-runtime`, `dist`, active source changes, the whitelisted UID remapper target, and its offline Rust toolchain. Remove only confirmed reproducible caches and build outputs, then verify the retained executable and Git working tree.

## Tasks

- [x] Inventory tracked, untracked, ignored, runtime, build, cache, and archive paths.
- [x] Add ignore boundaries for `.cache`, `.superpowers`, and `local-artifacts`.
- [x] Move root-level ZIP files into categorized `local-artifacts` directories without opening them.
- [x] Remove superseded task caches, duplicate Rust targets, and reproducible build output.
- [x] Verify protected paths remain, compute reclaimed space, inspect Git status, and confirm the retained UID remapper build remains usable.

## Protected Paths

- `dev-runtime/`
- `dist/`
- `.cache/task-5b/cargo/`
- `.cache/task-5b/rustup/`
- `.cache/task-5b/target-whitelisted/`
- `.superpowers/`
- All tracked and untracked source files under `backend/`, `frontend/`, `scripts/`, `third_party/`, and `tools/`

No commit is created because the user did not request one.
