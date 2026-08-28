# Dependency Audit Report

## Summary

- Root: `personal-portfolio`
- Packages scanned: 185
- Policy violations: 4

## Dependency Graph

```mermaid
graph TD
    n0["@babel/code-frame@7.29.7"]
    n1["@babel/code-frame@^7.29.7"]
    n2["@babel/compat-data@7.29.7"]
    n3["@babel/compat-data@^7.29.7"]
    n4["@babel/core@7.29.7"]
    n5["@babel/core@^7.28.0"]
    n6["@babel/generator@7.29.8"]
    n7["@babel/generator@^7.29.7"]
    n8["@babel/generator@^7.29.8"]
    n9["@babel/helper-compilation-targets@7.29.7"]
    n10["@babel/helper-compilation-targets@^7.29.7"]
    n11["@babel/helper-globals@7.29.7"]
    n12["@babel/helper-globals@^7.29.7"]
    n13["@babel/helper-module-imports@7.29.7"]
    n14["@babel/helper-module-imports@^7.29.7"]
    n15["@babel/helper-module-transforms@7.29.7"]
    n16["@babel/helper-module-transforms@^7.29.7"]
    n17["@babel/helper-plugin-utils@7.29.7"]
    n18["@babel/helper-plugin-utils@^7.29.7"]
    n19["@babel/helper-string-parser@7.29.7"]
    n20["@babel/helper-string-parser@^7.29.7"]
    n21["@babel/helper-validator-identifier@7.29.7"]
    n22["@babel/helper-validator-identifier@^7.29.7"]
    n23["@babel/helper-validator-option@7.29.7"]
    n24["@babel/helper-validator-option@^7.29.7"]
    n25["@babel/helpers@7.29.7"]
    n26["@babel/helpers@^7.29.7"]
    n27["@babel/parser@7.29.8"]
    n28["@babel/parser@^7.1.0"]
    n29["@babel/parser@^7.20.7"]
    n30["@babel/parser@^7.29.7"]
    n31["@babel/parser@^7.29.8"]
    n32["@babel/plugin-transform-react-jsx-self@7.29.7"]
    n33["@babel/plugin-transform-react-jsx-self@^7.27.1"]
    n34["@babel/plugin-transform-react-jsx-source@7.29.7"]
    n35["@babel/plugin-transform-react-jsx-source@^7.27.1"]
    n36["@babel/template@7.29.7"]
    n37["@babel/template@^7.29.7"]
    n38["@babel/traverse@7.29.8"]
    n39["@babel/traverse@^7.29.7"]
    n40["@babel/types@7.29.8"]
    n41["@babel/types@^7.0.0"]
    n42["@babel/types@^7.20.7"]
    n43["@babel/types@^7.28.2"]
    n44["@babel/types@^7.29.7"]
    n45["@babel/types@^7.29.8"]
    n46["@eslint-community/eslint-utils@4.10.1"]
    n47["@eslint-community/eslint-utils@^4.8.0"]
    n48["@eslint-community/eslint-utils@^4.9.1"]
    n49["@eslint-community/regexpp@4.12.2"]
    n50["@eslint-community/regexpp@^4.12.1"]
    n51["@eslint-community/regexpp@^4.12.2"]
    n52["@eslint/config-array@0.21.2"]
    n53["@eslint/config-array@^0.21.2"]
    n54["@eslint/config-helpers@0.4.2"]
    n55["@eslint/config-helpers@^0.4.2"]
    n56["@eslint/core@0.17.0"]
    n57["@eslint/core@^0.17.0"]
    n58["@eslint/eslintrc@3.3.6"]
    n59["@eslint/eslintrc@^3.3.6"]
    n60["@eslint/js@9.39.5"]
    n61["@eslint/object-schema@2.1.7"]
    n62["@eslint/object-schema@^2.1.7"]
    n63["@eslint/plugin-kit@0.4.1"]
    n64["@eslint/plugin-kit@^0.4.1"]
    n65["@humanfs/core@0.19.2"]
    n66["@humanfs/core@^0.19.2"]
    n67["@humanfs/node@0.16.8"]
    n68["@humanfs/node@^0.16.6"]
    n69["@humanfs/types@0.15.0"]
    n70["@humanfs/types@^0.15.0"]
    n71["@humanwhocodes/module-importer@1.0.1"]
    n72["@humanwhocodes/module-importer@^1.0.1"]
    n73["@humanwhocodes/retry@0.4.3"]
    n74["@humanwhocodes/retry@^0.4.0"]
    n75["@humanwhocodes/retry@^0.4.2"]
    n76["@jridgewell/gen-mapping@0.3.13"]
    n77["@jridgewell/gen-mapping@^0.3.12"]
    n78["@jridgewell/gen-mapping@^0.3.5"]
    n79["@jridgewell/remapping@2.3.5"]
    n80["@jridgewell/remapping@^2.3.5"]
    n81["@jridgewell/resolve-uri@3.1.2"]
    n82["@jridgewell/resolve-uri@^3.1.0"]
    n83["@jridgewell/sourcemap-codec@1.6.0"]
    n84["@jridgewell/sourcemap-codec@^1.4.14"]
    n85["@jridgewell/sourcemap-codec@^1.5.0"]
    n86["@jridgewell/sourcemap-codec@^1.5.5"]
    n87["@jridgewell/trace-mapping@0.3.31"]
    n88["@jridgewell/trace-mapping@^0.3.24"]
    n89["@jridgewell/trace-mapping@^0.3.28"]
    n90["@rolldown/pluginutils@1.0.0-beta.27"]
    n91["@tailwindcss/node@4.3.3"]
    n92["@tailwindcss/oxide@4.3.3"]
    n93["@tailwindcss/vite@4.3.3"]
    n94["@types/babel__core@7.20.5"]
    n95["@types/babel__core@^7.20.5"]
    n96["@types/babel__generator@*"]
    n97["@types/babel__generator@7.27.0"]
    n98["@types/babel__template@*"]
    n99["@types/babel__template@7.4.4"]
    n100["@types/babel__traverse@*"]
    n101["@types/babel__traverse@7.28.0"]
    n102["@types/estree@1.0.9"]
    n103["@types/estree@^1.0.6"]
    n104["@types/json-schema@7.0.15"]
    n105["@types/json-schema@^7.0.15"]
    n106["@types/react@19.2.18"]
    n107["@types/react-dom@19.2.5"]
    n108["@typescript-eslint/eslint-plugin@8.68.0"]
    n109["@typescript-eslint/parser@8.68.0"]
    n110["@typescript-eslint/project-service@8.68.0"]
    n111["@typescript-eslint/scope-manager@8.68.0"]
    n112["@typescript-eslint/tsconfig-utils@8.68.0"]
    n113["@typescript-eslint/tsconfig-utils@^8.68.0"]
    n114["@typescript-eslint/type-utils@8.68.0"]
    n115["@typescript-eslint/types@8.68.0"]
    n116["@typescript-eslint/types@^8.68.0"]
    n117["@typescript-eslint/typescript-estree@8.68.0"]
    n118["@typescript-eslint/utils@8.68.0"]
    n119["@typescript-eslint/visitor-keys@8.68.0"]
    n120["@vitejs/plugin-react@4.7.0"]
    n121["acorn@8.18.0"]
    n122["acorn@^8.15.0"]
    n123["acorn-jsx@5.3.2"]
    n124["acorn-jsx@^5.3.2"]
    n125["ajv@6.15.0"]
    n126["ajv@^6.14.0"]
    n127["ansi-styles@4.3.0"]
    n128["ansi-styles@^4.1.0"]
    n129["argparse@2.0.1"]
    n130["argparse@^2.0.1"]
    n131["balanced-match@1.0.2"]
    n132["balanced-match@4.0.4"]
    n133["balanced-match@^1.0.0"]
    n134["balanced-match@^4.0.2"]
    n135["baseline-browser-mapping@2.11.20"]
    n136["baseline-browser-mapping@^2.11.12"]
    n137["brace-expansion@1.1.18"]
    n138["brace-expansion@5.0.9"]
    n139["brace-expansion@^1.1.7"]
    n140["brace-expansion@^5.0.8"]
    n141["browserslist@4.28.8"]
    n142["browserslist@^4.24.0"]
    n143["callsites@3.1.0"]
    n144["callsites@^3.0.0"]
    n145["caniuse-lite@1.0.30001810"]
    n146["caniuse-lite@^1.0.30001809"]
    n147["chalk@4.1.2"]
    n148["chalk@^4.0.0"]
    n149["color-convert@2.0.1"]
    n150["color-convert@^2.0.1"]
    n151["color-name@1.1.4"]
    n152["color-name@~1.1.4"]
    n153["concat-map@0.0.1"]
    n154["convert-source-map@2.0.0"]
    n155["convert-source-map@^2.0.0"]
    n156["cookie@1.1.1"]
    n157["cookie@^1.0.1"]
    n158["cross-spawn@7.0.6"]
    n159["cross-spawn@^7.0.6"]
    n160["csstype@3.2.3"]
    n161["csstype@^3.2.2"]
    n162["debug@4.4.3"]
    n163["debug@^4.1.0"]
    n164["debug@^4.3.1"]
    n165["debug@^4.3.2"]
    n166["debug@^4.4.3"]
    n167["deep-is@0.1.4"]
    n168["deep-is@^0.1.3"]
    n169["detect-libc@2.1.2"]
    n170["detect-libc@^2.0.3"]
    n171["electron-to-chromium@1.5.416"]
    n172["electron-to-chromium@^1.5.402"]
    n173["enhanced-resolve@5.24.5"]
    n174["enhanced-resolve@^5.24.1"]
    n175["esbuild@0.25.12"]
    n176["esbuild@^0.25.0"]
    n177["escalade@3.2.0"]
    n178["escalade@^3.2.0"]
    n179["escape-string-regexp@4.0.0"]
    n180["escape-string-regexp@^4.0.0"]
    n181["eslint@9.39.5"]
    n182["eslint-plugin-react-hooks@5.2.0"]
    n183["eslint-plugin-react-refresh@0.4.26"]
    n184["eslint-scope@8.4.0"]
    n185["eslint-scope@^8.4.0"]
    n186["eslint-visitor-keys@3.4.3"]
    n187["eslint-visitor-keys@4.2.1"]
    n188["eslint-visitor-keys@5.0.1"]
    n189["eslint-visitor-keys@^3.4.3"]
    n190["eslint-visitor-keys@^4.2.1"]
    n191["eslint-visitor-keys@^5.0.0"]
    n192["espree@10.4.0"]
    n193["espree@^10.0.1"]
    n194["espree@^10.4.0"]
    n195["esquery@1.7.0"]
    n196["esquery@^1.5.0"]
    n197["esrecurse@4.3.0"]
    n198["esrecurse@^4.3.0"]
    n199["estraverse@5.3.0"]
    n200["estraverse@^5.1.0"]
    n201["estraverse@^5.2.0"]
    n202["esutils@2.0.3"]
    n203["esutils@^2.0.2"]
    n204["fast-deep-equal@3.1.3"]
    n205["fast-deep-equal@^3.1.1"]
    n206["fast-deep-equal@^3.1.3"]
    n207["fast-json-stable-stringify@2.1.0"]
    n208["fast-json-stable-stringify@^2.0.0"]
    n209["fast-levenshtein@2.0.6"]
    n210["fast-levenshtein@^2.0.6"]
    n211["fdir@6.5.0"]
    n212["fdir@^6.4.4"]
    n213["fdir@^6.5.0"]
    n214["file-entry-cache@8.0.0"]
    n215["file-entry-cache@^8.0.0"]
    n216["find-up@5.0.0"]
    n217["find-up@^5.0.0"]
    n218["flat-cache@4.0.1"]
    n219["flat-cache@^4.0.0"]
    n220["flatted@3.4.4"]
    n221["flatted@^3.2.9"]
    n222["gensync@1.0.0-beta.2"]
    n223["gensync@^1.0.0-beta.2"]
    n224["glob-parent@6.0.2"]
    n225["glob-parent@^6.0.2"]
    n226["globals@14.0.0"]
    n227["globals@16.5.0"]
    n228["globals@^14.0.0"]
    n229["graceful-fs@4.2.11"]
    n230["graceful-fs@^4.2.4"]
    n231["has-flag@4.0.0"]
    n232["has-flag@^4.0.0"]
    n233["ignore@5.3.2"]
    n234["ignore@7.0.6"]
    n235["ignore@^5.2.0"]
    n236["ignore@^7.0.5"]
    n237["import-fresh@3.3.1"]
    n238["import-fresh@^3.2.1"]
    n239["imurmurhash@0.1.4"]
    n240["imurmurhash@^0.1.4"]
    n241["is-extglob@2.1.1"]
    n242["is-extglob@^2.1.1"]
    n243["is-glob@4.0.3"]
    n244["is-glob@^4.0.0"]
    n245["is-glob@^4.0.3"]
    n246["isexe@2.0.0"]
    n247["isexe@^2.0.0"]
    n248["jiti@2.7.0"]
    n249["jiti@^2.7.0"]
    n250["js-tokens@4.0.0"]
    n251["js-tokens@^4.0.0"]
    n252["js-yaml@4.3.2"]
    n253["js-yaml@^4.3.0"]
    n254["jsesc@3.1.0"]
    n255["jsesc@^3.0.2"]
    n256["json-buffer@3.0.1"]
    n257["json-schema-traverse@0.4.1"]
    n258["json-schema-traverse@^0.4.1"]
    n259["json-stable-stringify-without-jsonify@1.0.1"]
    n260["json-stable-stringify-without-jsonify@^1.0.1"]
    n261["json5@2.2.3"]
    n262["json5@^2.2.3"]
    n263["keyv@4.5.4"]
    n264["keyv@^4.5.4"]
    n265["levn@0.4.1"]
    n266["levn@^0.4.1"]
    n267["lightningcss@1.32.0"]
    n268["locate-path@6.0.0"]
    n269["locate-path@^6.0.0"]
    n270["lodash.merge@4.6.2"]
    n271["lodash.merge@^4.6.2"]
    n272["lru-cache@5.1.1"]
    n273["lru-cache@^5.1.1"]
    n274["magic-string@0.30.21"]
    n275["magic-string@^0.30.21"]
    n276["minimatch@10.2.6"]
    n277["minimatch@3.1.5"]
    n278["minimatch@^10.2.2"]
    n279["minimatch@^3.1.5"]
    n280["ms@2.1.3"]
    n281["ms@^2.1.3"]
    n282["nanoid@3.3.18"]
    n283["nanoid@^3.3.17"]
    n284["natural-compare@1.4.0"]
    n285["natural-compare@^1.4.0"]
    n286["node-releases@2.0.54"]
    n287["node-releases@^2.0.53"]
    n288["optionator@0.9.4"]
    n289["optionator@^0.9.3"]
    n290["p-limit@3.1.0"]
    n291["p-limit@^3.0.2"]
    n292["p-locate@5.0.0"]
    n293["p-locate@^5.0.0"]
    n294["parent-module@1.0.1"]
    n295["parent-module@^1.0.0"]
    n296["path-exists@4.0.0"]
    n297["path-exists@^4.0.0"]
    n298["path-key@3.1.1"]
    n299["path-key@^3.1.0"]
    n300["picocolors@1.1.1"]
    n301["picocolors@^1.1.1"]
    n302["picomatch@4.0.7"]
    n303["picomatch@^4.0.2"]
    n304["picomatch@^4.0.4"]
    n305["postcss@8.5.26"]
    n306["postcss@^8.5.3"]
    n307["prelude-ls@1.2.1"]
    n308["prelude-ls@^1.2.1"]
    n309["punycode@2.3.1"]
    n310["punycode@^2.1.0"]
    n311["react@19.2.8"]
    n312["react-dom@19.2.8"]
    n313["react-refresh@0.17.0"]
    n314["react-refresh@^0.17.0"]
    n315["react-router@7.18.3"]
    n316["react-router-dom@7.18.3"]
    n317["resolve-from@4.0.0"]
    n318["resolve-from@^4.0.0"]
    n319["rollup@4.63.1"]
    n320["rollup@^4.34.9"]
    n321["scheduler@0.27.0"]
    n322["scheduler@^0.27.0"]
    n323["semver@6.3.1"]
    n324["semver@7.8.5"]
    n325["semver@^6.3.1"]
    n326["semver@^7.7.3"]
    n327["set-cookie-parser@2.7.2"]
    n328["set-cookie-parser@^2.6.0"]
    n329["shebang-command@2.0.0"]
    n330["shebang-command@^2.0.0"]
    n331["shebang-regex@3.0.0"]
    n332["shebang-regex@^3.0.0"]
    n333["source-map-js@1.2.1"]
    n334["source-map-js@^1.2.1"]
    n335["strip-json-comments@3.1.1"]
    n336["strip-json-comments@^3.1.1"]
    n337["supports-color@7.2.0"]
    n338["supports-color@^7.1.0"]
    n339["tailwindcss@4.3.3"]
    n340["tapable@2.3.3"]
    n341["tapable@^2.3.3"]
    n342["tinyglobby@0.2.17"]
    n343["tinyglobby@^0.2.13"]
    n344["tinyglobby@^0.2.15"]
    n345["ts-api-utils@2.5.0"]
    n346["ts-api-utils@^2.5.0"]
    n347["type-check@0.4.0"]
    n348["type-check@^0.4.0"]
    n349["type-check@~0.4.0"]
    n350["typescript@5.8.3"]
    n351["typescript-eslint@8.68.0"]
    n352["update-browserslist-db@1.3.2"]
    n353["update-browserslist-db@^1.3.0"]
    n354["uri-js@4.4.1"]
    n355["uri-js@^4.2.2"]
    n356["vite@6.4.3"]
    n357["which@2.0.2"]
    n358["which@^2.0.1"]
    n359["word-wrap@1.2.5"]
    n360["word-wrap@^1.2.5"]
    n361["yallist@3.1.1"]
    n362["yallist@^3.0.2"]
    n363["yocto-queue@0.1.0"]
    n364["yocto-queue@^0.1.0"]
    n0 --> n22
    n0 --> n251
    n0 --> n301
    n4 --> n1
    n4 --> n7
    n4 --> n10
    n4 --> n16
    n4 --> n26
    n4 --> n30
    n4 --> n37
    n4 --> n39
    n4 --> n44
    n4 --> n80
    n4 --> n155
    n4 --> n163
    n4 --> n223
    n4 --> n262
    n4 --> n325
    n6 --> n31
    n6 --> n45
    n6 --> n77
    n6 --> n89
    n6 --> n255
    n9 --> n3
    n9 --> n24
    n9 --> n142
    n9 --> n273
    n9 --> n325
    n13 --> n39
    n13 --> n44
    n15 --> n14
    n15 --> n22
    n15 --> n39
    n25 --> n37
    n25 --> n44
    n27 --> n45
    n32 --> n18
    n34 --> n18
    n36 --> n1
    n36 --> n30
    n36 --> n44
    n38 --> n1
    n38 --> n8
    n38 --> n12
    n38 --> n31
    n38 --> n37
    n38 --> n45
    n38 --> n164
    n40 --> n20
    n40 --> n22
    n46 --> n189
    n52 --> n62
    n52 --> n164
    n52 --> n279
    n54 --> n57
    n56 --> n105
    n58 --> n126
    n58 --> n165
    n58 --> n193
    n58 --> n228
    n58 --> n235
    n58 --> n238
    n58 --> n253
    n58 --> n279
    n58 --> n336
    n63 --> n57
    n63 --> n266
    n65 --> n70
    n67 --> n66
    n67 --> n70
    n67 --> n74
    n76 --> n85
    n76 --> n88
    n79 --> n78
    n79 --> n88
    n87 --> n82
    n87 --> n84
    n91 --> n80
    n91 --> n174
    n91 --> n249
    n91 --> n267
    n91 --> n275
    n91 --> n334
    n91 --> n339
    n93 --> n91
    n93 --> n92
    n93 --> n339
    n94 --> n29
    n94 --> n42
    n94 --> n96
    n94 --> n98
    n94 --> n100
    n97 --> n41
    n99 --> n28
    n99 --> n41
    n101 --> n43
    n106 --> n161
    n108 --> n51
    n108 --> n111
    n108 --> n114
    n108 --> n118
    n108 --> n119
    n108 --> n236
    n108 --> n285
    n108 --> n346
    n109 --> n111
    n109 --> n115
    n109 --> n117
    n109 --> n119
    n109 --> n166
    n110 --> n113
    n110 --> n116
    n110 --> n166
    n111 --> n115
    n111 --> n119
    n114 --> n115
    n114 --> n117
    n114 --> n118
    n114 --> n166
    n114 --> n346
    n117 --> n110
    n117 --> n112
    n117 --> n115
    n117 --> n119
    n117 --> n166
    n117 --> n278
    n117 --> n326
    n117 --> n344
    n117 --> n346
    n118 --> n48
    n118 --> n111
    n118 --> n115
    n118 --> n117
    n119 --> n115
    n119 --> n191
    n120 --> n5
    n120 --> n33
    n120 --> n35
    n120 --> n90
    n120 --> n95
    n120 --> n314
    n125 --> n205
    n125 --> n208
    n125 --> n258
    n125 --> n355
    n127 --> n150
    n137 --> n133
    n137 --> n153
    n138 --> n134
    n141 --> n136
    n141 --> n146
    n141 --> n172
    n141 --> n287
    n141 --> n353
    n147 --> n128
    n147 --> n338
    n149 --> n152
    n158 --> n299
    n158 --> n330
    n158 --> n358
    n162 --> n281
    n173 --> n230
    n173 --> n341
    n181 --> n47
    n181 --> n50
    n181 --> n53
    n181 --> n55
    n181 --> n57
    n181 --> n59
    n181 --> n60
    n181 --> n64
    n181 --> n68
    n181 --> n72
    n181 --> n75
    n181 --> n103
    n181 --> n126
    n181 --> n148
    n181 --> n159
    n181 --> n165
    n181 --> n180
    n181 --> n185
    n181 --> n190
    n181 --> n194
    n181 --> n196
    n181 --> n203
    n181 --> n206
    n181 --> n215
    n181 --> n217
    n181 --> n225
    n181 --> n235
    n181 --> n240
    n181 --> n244
    n181 --> n260
    n181 --> n271
    n181 --> n279
    n181 --> n285
    n181 --> n289
    n184 --> n198
    n184 --> n201
    n192 --> n122
    n192 --> n124
    n192 --> n190
    n195 --> n200
    n197 --> n201
    n214 --> n219
    n216 --> n269
    n216 --> n297
    n218 --> n221
    n218 --> n264
    n224 --> n245
    n237 --> n295
    n237 --> n318
    n243 --> n242
    n252 --> n130
    n263 --> n256
    n265 --> n308
    n265 --> n349
    n267 --> n170
    n268 --> n293
    n272 --> n362
    n274 --> n86
    n276 --> n140
    n277 --> n139
    n288 --> n168
    n288 --> n210
    n288 --> n266
    n288 --> n308
    n288 --> n348
    n288 --> n360
    n290 --> n364
    n292 --> n291
    n294 --> n144
    n305 --> n283
    n305 --> n301
    n305 --> n334
    n312 --> n322
    n315 --> n157
    n315 --> n328
    n316 --> n315
    n319 --> n102
    n329 --> n332
    n337 --> n232
    n342 --> n213
    n342 --> n304
    n347 --> n308
    n351 --> n108
    n351 --> n109
    n351 --> n117
    n351 --> n118
    n352 --> n178
    n352 --> n301
    n354 --> n310
    n356 --> n176
    n356 --> n212
    n356 --> n303
    n356 --> n306
    n356 --> n320
    n356 --> n343
    n357 --> n247
```

## Packages

| Package | Version | License | Verdict |
|---|---|---|---|
| `@babel/code-frame` | `7.29.7` | `MIT` | `pass` |
| `@babel/compat-data` | `7.29.7` | `MIT` | `pass` |
| `@babel/core` | `7.29.7` | `MIT` | `pass` |
| `@babel/generator` | `7.29.8` | `MIT` | `pass` |
| `@babel/helper-compilation-targets` | `7.29.7` | `MIT` | `pass` |
| `@babel/helper-globals` | `7.29.7` | `MIT` | `pass` |
| `@babel/helper-module-imports` | `7.29.7` | `MIT` | `pass` |
| `@babel/helper-module-transforms` | `7.29.7` | `MIT` | `pass` |
| `@babel/helper-plugin-utils` | `7.29.7` | `MIT` | `pass` |
| `@babel/helper-string-parser` | `7.29.7` | `MIT` | `pass` |
| `@babel/helper-validator-identifier` | `7.29.7` | `MIT` | `pass` |
| `@babel/helper-validator-option` | `7.29.7` | `MIT` | `pass` |
| `@babel/helpers` | `7.29.7` | `MIT` | `pass` |
| `@babel/parser` | `7.29.8` | `MIT` | `pass` |
| `@babel/plugin-transform-react-jsx-self` | `7.29.7` | `MIT` | `pass` |
| `@babel/plugin-transform-react-jsx-source` | `7.29.7` | `MIT` | `pass` |
| `@babel/template` | `7.29.7` | `MIT` | `pass` |
| `@babel/traverse` | `7.29.8` | `MIT` | `pass` |
| `@babel/types` | `7.29.8` | `MIT` | `pass` |
| `@eslint-community/eslint-utils` | `4.10.1` | `MIT` | `pass` |
| `@eslint-community/regexpp` | `4.12.2` | `MIT` | `pass` |
| `@eslint/config-array` | `0.21.2` | `Apache-2.0` | `pass` |
| `@eslint/config-helpers` | `0.4.2` | `Apache-2.0` | `pass` |
| `@eslint/core` | `0.17.0` | `Apache-2.0` | `pass` |
| `@eslint/eslintrc` | `3.3.6` | `MIT` | `pass` |
| `@eslint/js` | `9.39.5` | `MIT` | `pass` |
| `@eslint/object-schema` | `2.1.7` | `Apache-2.0` | `pass` |
| `@eslint/plugin-kit` | `0.4.1` | `Apache-2.0` | `pass` |
| `@humanfs/core` | `0.19.2` | `Apache-2.0` | `pass` |
| `@humanfs/node` | `0.16.8` | `Apache-2.0` | `pass` |
| `@humanfs/types` | `0.15.0` | `Apache-2.0` | `pass` |
| `@humanwhocodes/module-importer` | `1.0.1` | `Apache-2.0` | `pass` |
| `@humanwhocodes/retry` | `0.4.3` | `Apache-2.0` | `pass` |
| `@jridgewell/gen-mapping` | `0.3.13` | `MIT` | `pass` |
| `@jridgewell/remapping` | `2.3.5` | `MIT` | `pass` |
| `@jridgewell/resolve-uri` | `3.1.2` | `MIT` | `pass` |
| `@jridgewell/sourcemap-codec` | `1.6.0` | `MIT` | `pass` |
| `@jridgewell/trace-mapping` | `0.3.31` | `MIT` | `pass` |
| `@rolldown/pluginutils` | `1.0.0-beta.27` | `MIT` | `pass` |
| `@tailwindcss/node` | `4.3.3` | `MIT` | `pass` |
| `@tailwindcss/oxide` | `4.3.3` | `MIT` | `pass` |
| `@tailwindcss/vite` | `4.3.3` | `MIT` | `pass` |
| `@types/babel__core` | `7.20.5` | `MIT` | `pass` |
| `@types/babel__generator` | `7.27.0` | `MIT` | `pass` |
| `@types/babel__template` | `7.4.4` | `MIT` | `pass` |
| `@types/babel__traverse` | `7.28.0` | `MIT` | `pass` |
| `@types/estree` | `1.0.9` | `MIT` | `pass` |
| `@types/json-schema` | `7.0.15` | `MIT` | `pass` |
| `@types/react` | `19.2.18` | `MIT` | `pass` |
| `@types/react-dom` | `19.2.5` | `MIT` | `pass` |
| `@typescript-eslint/eslint-plugin` | `8.68.0` | `MIT` | `pass` |
| `@typescript-eslint/parser` | `8.68.0` | `MIT` | `pass` |
| `@typescript-eslint/project-service` | `8.68.0` | `MIT` | `pass` |
| `@typescript-eslint/scope-manager` | `8.68.0` | `MIT` | `pass` |
| `@typescript-eslint/tsconfig-utils` | `8.68.0` | `MIT` | `pass` |
| `@typescript-eslint/type-utils` | `8.68.0` | `MIT` | `pass` |
| `@typescript-eslint/types` | `8.68.0` | `MIT` | `pass` |
| `@typescript-eslint/typescript-estree` | `8.68.0` | `MIT` | `pass` |
| `@typescript-eslint/utils` | `8.68.0` | `MIT` | `pass` |
| `@typescript-eslint/visitor-keys` | `8.68.0` | `MIT` | `pass` |
| `@vitejs/plugin-react` | `4.7.0` | `MIT` | `pass` |
| `acorn` | `8.18.0` | `MIT` | `pass` |
| `acorn-jsx` | `5.3.2` | `MIT` | `pass` |
| `ajv` | `6.15.0` | `MIT` | `pass` |
| `ansi-styles` | `4.3.0` | `MIT` | `pass` |
| `argparse` | `2.0.1` | `Python-2.0` | `policy_violation` |
| `balanced-match` | `1.0.2` | `MIT` | `pass` |
| `balanced-match` | `4.0.4` | `MIT` | `pass` |
| `baseline-browser-mapping` | `2.11.20` | `Apache-2.0` | `pass` |
| `brace-expansion` | `1.1.18` | `MIT` | `pass` |
| `brace-expansion` | `5.0.9` | `MIT` | `pass` |
| `browserslist` | `4.28.8` | `MIT` | `pass` |
| `callsites` | `3.1.0` | `MIT` | `pass` |
| `caniuse-lite` | `1.0.30001810` | `CC-BY-4.0` | `policy_violation` |
| `chalk` | `4.1.2` | `MIT` | `pass` |
| `color-convert` | `2.0.1` | `MIT` | `pass` |
| `color-name` | `1.1.4` | `MIT` | `pass` |
| `concat-map` | `0.0.1` | `MIT` | `pass` |
| `convert-source-map` | `2.0.0` | `MIT` | `pass` |
| `cookie` | `1.1.1` | `MIT` | `pass` |
| `cross-spawn` | `7.0.6` | `MIT` | `pass` |
| `csstype` | `3.2.3` | `MIT` | `pass` |
| `debug` | `4.4.3` | `MIT` | `pass` |
| `deep-is` | `0.1.4` | `MIT` | `pass` |
| `detect-libc` | `2.1.2` | `Apache-2.0` | `pass` |
| `electron-to-chromium` | `1.5.416` | `ISC` | `pass` |
| `enhanced-resolve` | `5.24.5` | `MIT` | `pass` |
| `esbuild` | `0.25.12` | `MIT` | `pass` |
| `escalade` | `3.2.0` | `MIT` | `pass` |
| `escape-string-regexp` | `4.0.0` | `MIT` | `pass` |
| `eslint` | `9.39.5` | `MIT` | `pass` |
| `eslint-plugin-react-hooks` | `5.2.0` | `MIT` | `pass` |
| `eslint-plugin-react-refresh` | `0.4.26` | `MIT` | `pass` |
| `eslint-scope` | `8.4.0` | `BSD-2-Clause` | `pass` |
| `eslint-visitor-keys` | `3.4.3` | `Apache-2.0` | `pass` |
| `eslint-visitor-keys` | `4.2.1` | `Apache-2.0` | `pass` |
| `eslint-visitor-keys` | `5.0.1` | `Apache-2.0` | `pass` |
| `espree` | `10.4.0` | `BSD-2-Clause` | `pass` |
| `esquery` | `1.7.0` | `BSD-3-Clause` | `pass` |
| `esrecurse` | `4.3.0` | `BSD-2-Clause` | `pass` |
| `estraverse` | `5.3.0` | `BSD-2-Clause` | `pass` |
| `esutils` | `2.0.3` | `BSD-2-Clause` | `pass` |
| `fast-deep-equal` | `3.1.3` | `MIT` | `pass` |
| `fast-json-stable-stringify` | `2.1.0` | `MIT` | `pass` |
| `fast-levenshtein` | `2.0.6` | `MIT` | `pass` |
| `fdir` | `6.5.0` | `MIT` | `pass` |
| `file-entry-cache` | `8.0.0` | `MIT` | `pass` |
| `find-up` | `5.0.0` | `MIT` | `pass` |
| `flat-cache` | `4.0.1` | `MIT` | `pass` |
| `flatted` | `3.4.4` | `ISC` | `pass` |
| `gensync` | `1.0.0-beta.2` | `MIT` | `pass` |
| `glob-parent` | `6.0.2` | `ISC` | `pass` |
| `globals` | `14.0.0` | `MIT` | `pass` |
| `globals` | `16.5.0` | `MIT` | `pass` |
| `graceful-fs` | `4.2.11` | `ISC` | `pass` |
| `has-flag` | `4.0.0` | `MIT` | `pass` |
| `ignore` | `5.3.2` | `MIT` | `pass` |
| `ignore` | `7.0.6` | `MIT` | `pass` |
| `import-fresh` | `3.3.1` | `MIT` | `pass` |
| `imurmurhash` | `0.1.4` | `MIT` | `pass` |
| `is-extglob` | `2.1.1` | `MIT` | `pass` |
| `is-glob` | `4.0.3` | `MIT` | `pass` |
| `isexe` | `2.0.0` | `ISC` | `pass` |
| `jiti` | `2.7.0` | `MIT` | `pass` |
| `js-tokens` | `4.0.0` | `MIT` | `pass` |
| `js-yaml` | `4.3.2` | `MIT` | `pass` |
| `jsesc` | `3.1.0` | `MIT` | `pass` |
| `json-buffer` | `3.0.1` | `MIT` | `pass` |
| `json-schema-traverse` | `0.4.1` | `MIT` | `pass` |
| `json-stable-stringify-without-jsonify` | `1.0.1` | `MIT` | `pass` |
| `json5` | `2.2.3` | `MIT` | `pass` |
| `keyv` | `4.5.4` | `MIT` | `pass` |
| `levn` | `0.4.1` | `MIT` | `pass` |
| `lightningcss` | `1.32.0` | `MPL-2.0` | `policy_violation` |
| `locate-path` | `6.0.0` | `MIT` | `pass` |
| `lodash.merge` | `4.6.2` | `MIT` | `pass` |
| `lru-cache` | `5.1.1` | `ISC` | `pass` |
| `magic-string` | `0.30.21` | `MIT` | `pass` |
| `minimatch` | `10.2.6` | `BlueOak-1.0.0` | `policy_violation` |
| `minimatch` | `3.1.5` | `ISC` | `pass` |
| `ms` | `2.1.3` | `MIT` | `pass` |
| `nanoid` | `3.3.18` | `MIT` | `pass` |
| `natural-compare` | `1.4.0` | `MIT` | `pass` |
| `node-releases` | `2.0.54` | `MIT` | `pass` |
| `optionator` | `0.9.4` | `MIT` | `pass` |
| `p-limit` | `3.1.0` | `MIT` | `pass` |
| `p-locate` | `5.0.0` | `MIT` | `pass` |
| `parent-module` | `1.0.1` | `MIT` | `pass` |
| `path-exists` | `4.0.0` | `MIT` | `pass` |
| `path-key` | `3.1.1` | `MIT` | `pass` |
| `picocolors` | `1.1.1` | `ISC` | `pass` |
| `picomatch` | `4.0.7` | `MIT` | `pass` |
| `postcss` | `8.5.26` | `MIT` | `pass` |
| `prelude-ls` | `1.2.1` | `MIT` | `pass` |
| `punycode` | `2.3.1` | `MIT` | `pass` |
| `react` | `19.2.8` | `MIT` | `pass` |
| `react-dom` | `19.2.8` | `MIT` | `pass` |
| `react-refresh` | `0.17.0` | `MIT` | `pass` |
| `react-router` | `7.18.3` | `MIT` | `pass` |
| `react-router-dom` | `7.18.3` | `MIT` | `pass` |
| `resolve-from` | `4.0.0` | `MIT` | `pass` |
| `rollup` | `4.63.1` | `MIT` | `pass` |
| `scheduler` | `0.27.0` | `MIT` | `pass` |
| `semver` | `6.3.1` | `ISC` | `pass` |
| `semver` | `7.8.5` | `ISC` | `pass` |
| `set-cookie-parser` | `2.7.2` | `MIT` | `pass` |
| `shebang-command` | `2.0.0` | `MIT` | `pass` |
| `shebang-regex` | `3.0.0` | `MIT` | `pass` |
| `source-map-js` | `1.2.1` | `BSD-3-Clause` | `pass` |
| `strip-json-comments` | `3.1.1` | `MIT` | `pass` |
| `supports-color` | `7.2.0` | `MIT` | `pass` |
| `tailwindcss` | `4.3.3` | `MIT` | `pass` |
| `tapable` | `2.3.3` | `MIT` | `pass` |
| `tinyglobby` | `0.2.17` | `MIT` | `pass` |
| `ts-api-utils` | `2.5.0` | `MIT` | `pass` |
| `type-check` | `0.4.0` | `MIT` | `pass` |
| `typescript` | `5.8.3` | `Apache-2.0` | `pass` |
| `typescript-eslint` | `8.68.0` | `MIT` | `pass` |
| `update-browserslist-db` | `1.3.2` | `MIT` | `pass` |
| `uri-js` | `4.4.1` | `BSD-2-Clause` | `pass` |
| `vite` | `6.4.3` | `MIT` | `pass` |
| `which` | `2.0.2` | `ISC` | `pass` |
| `word-wrap` | `1.2.5` | `MIT` | `pass` |
| `yallist` | `3.1.1` | `ISC` | `pass` |
| `yocto-queue` | `0.1.0` | `MIT` | `pass` |

## Policy Violations

| Package | License | Dependency path |
|---|---|---|
| `argparse@2.0.1` | `Python-2.0` | `argparse@2.0.1` |
| `caniuse-lite@1.0.30001810` | `CC-BY-4.0` | `caniuse-lite@1.0.30001810` |
| `lightningcss@1.32.0` | `MPL-2.0` | `lightningcss@1.32.0` |
| `minimatch@10.2.6` | `BlueOak-1.0.0` | `minimatch@10.2.6` |
