# astrbot_plugin_palpanel

AstrBot 4.18+ OneBot v11/NapCat plugin for PalPanel. Runtime data is stored in
`data/plugin_data/astrbot_plugin_palpanel/palpanel.sqlite3`.

Member commands: `/bd <游戏昵称>`, `/bdqr <验证码>`, `/qd`, `/jf`, `/pz`, and
`/pz <目标帕鲁> [被动词条...]`. The latter reserves points, waits for the
PalCalc result, returns a route summary, and includes a one-time restricted web
link.

Administrators configured in `admin_qq_ids` can use `/paladmin` for manual
binding, unbinding/freezing, point adjustments, and ledger lookup. Every such
operation is audited.

Configure the same HMAC secret in AstrBot and `PALPANEL_ASTRBOT_SHARED_SECRET`. The internal API listens on loopback by default.
