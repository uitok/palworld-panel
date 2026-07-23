# Palworld Chinese name catalog

`catalog.zh-CN.json` is a modified, reduced catalog derived from
[`zaigie/palworld-server-tool`](https://github.com/zaigie/palworld-server-tool)
commit `18df587bd9e62d0f890b8cef1c32985fa6e9ba39`.

The upstream NOTICE attribution is preserved below:

```text
palworld-server-tool
Copyright 2024 zaigie
```

Only simplified-Chinese Pal, item, passive-skill, and technology display
metadata plus item icon keys are retained. The 115 standard passive-skill names
are generated from the MIT-licensed PalCalc `v26` database pinned at commit
`8b7e2f779e47fddae16ddcb973e828ba20c02b80` by
`scripts/generate-palcalc-passives.go`. The technology list is reduced from
`bingyouxue/palworld-server-tool-gm` commit
`d45c74cf92ca3d1b081bf03a62adfebe131888ad` to TechID, Chinese name, level,
category, ancient-technology status, and the upstream OP.GG identification-icon
URL. The images themselves and descriptions are not bundled. Other languages were removed, keys were normalized at runtime,
and the previous PalPanel item-name overrides were preserved while the
Palworld 1.0 catalog was expanded to 2,455 ItemIDs.

The source project is licensed under Apache License 2.0. A copy is included in
`LICENSE.apache-2.0`.

PalCalc is licensed under MIT. Its complete license text is included in the
release inventory as `licenses/PalCalc-MIT.txt`.

The item WebP files are game artwork used for identification. Palworld and its
artwork are property of Pocketpair, Inc.; they are not relicensed by PalPanel's
GPL or the source project's Apache-2.0 license.
