# Palworld Chinese name catalog

`catalog.zh-CN.json` is a modified, reduced catalog derived from
[`zaigie/palworld-server-tool`](https://github.com/zaigie/palworld-server-tool)
commit `a1e0079f830bc03f94e359b6f4a68ca2bd3b0187`.

Only simplified-Chinese Pal, item, and passive-skill display names are retained.
Descriptions and other languages were removed, keys were normalized at runtime,
and PalPanel-specific fallback names were added in `localize.go`.

The source project is licensed under Apache License 2.0. A copy is included in
`LICENSE.apache-2.0`.
