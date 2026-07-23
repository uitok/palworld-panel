# Workspace Layout

PalPanel keeps product source, vendored dependencies, generated output, runtime data, and local-only references in separate top-level paths.

| Path | Purpose | Versioned |
| --- | --- | --- |
| `backend/` | Go API, server lifecycle, persistence, monitoring, and tests | Yes |
| `frontend/` | React client, unit tests, and browser tests | Yes |
| `astrbot_plugin_palpanel/` | AstrBot integration | Yes |
| `palcalc-bridge/` | PalCalc sidecar source | Yes |
| `sav-cli/` | Save parsing command source | Yes |
| `tools/` | First-party maintenance and migration tools | Yes |
| `third_party/` | Pinned upstream source and provenance | Yes |
| `scripts/` | Packaging, installation, verification, and service scripts | Yes |
| `docs/` | Product and engineering documentation | Yes |
| `dist/` | Reproducible release packages | No |
| `output/` | Test and deployment output | No |
| `dev-runtime/` | Local server, SteamCMD, saves, and runtime state | No |
| `.cache/` | Local compiler, package, and task caches | No |
| `.superpowers/` | Local implementation progress and review notes | No |
| `local-artifacts/` | Local-only save archives and implementation references | No |

## Local Artifacts

Local ZIP files are grouped without extracting or inspecting their contents:

- `local-artifacts/save-archives/`: local save archives and manual validation inputs.
- `local-artifacts/migration-reference/`: migration toolkit references.
- `local-artifacts/frontend-reference/`: UI reference packages.

These paths are ignored by Git. Production saves, private UID mappings, and generated reports must not be copied into source, tests, logs, or release packages.
