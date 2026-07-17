# palcalc-bridge

Loopback-only HTTP bridge around the MIT-licensed PalCalc model and solver.

```powershell
dotnet run --project palcalc-bridge/PalCalc.Bridge.csproj
```

Default address: `http://127.0.0.1:8091`.

The service intentionally does not reference PalCalc's Windows-only save reader or WPF UI. PalPanel's Go `sav-cli` supplies normalized owned-pal data.
