# PalPanel sav_cli

Read-only Palworld `.sav` indexer sidecar written in Go.

```bash
cd sav-cli
go test ./...
go run ./cmd/sav_cli inspect --file ../data/server/Pal/Saved/SaveGames/0/<world>/Level.sav
go run ./cmd/sav_cli index --save-dir ../data/server/Pal/Saved/SaveGames
go run ./cmd/sav_cli serve --host 127.0.0.1 --port 8090
```

The sidecar exposes the same HTTP contract used by PalPanel:

- `GET /health`
- `POST /index` with `{ "save_dir": "/path/to/world-or-save-root" }`

Known status:

- `PlZ1`, `PlZ2`, and `CNK` containers are decoded with zlib.
- `PlM1` is decoded with the open-source `gooz` Oodle/Kraken decompressor when cgo is enabled.
- Static `CGO_ENABLED=0` builds report `parser_incompatible` for `PlM1` instead of failing to compile.
- The parser is read-only and never writes back to `.sav`.
