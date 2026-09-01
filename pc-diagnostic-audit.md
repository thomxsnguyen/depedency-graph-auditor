# Dependency Audit Report

## Summary

- Root: `pc-diagnostic`
- Python platform: `linux`
- Python version: `3.12`
- Packages scanned: 21
- Policy violations: 7

## Dependency Overview

```mermaid
flowchart TB
    n0["keyring@25.7.0"]
    n1["pc-diagnostic"]
    n2["pyqtgraph@0.14.0"]
    n3["pyside6@6.11.2"]
    n4["rich@15.0.0"]
    n1 --> n0
    n1 --> n2
    n1 --> n3
    n1 --> n4
```

## Violation Paths

### `cffi@2.1.1`

```mermaid
flowchart LR
    p0["pc-diagnostic"]
    p1["keyring@25.7.0"]
    p2["secretstorage@3.5.0"]
    p3["cryptography@50.0.1"]
    p4["cffi@2.1.1"]
    p0 --> p1
    p1 --> p2
    p2 --> p3
    p3 --> p4
```

### `cryptography@50.0.1`

```mermaid
flowchart LR
    p0["pc-diagnostic"]
    p1["keyring@25.7.0"]
    p2["secretstorage@3.5.0"]
    p3["cryptography@50.0.1"]
    p0 --> p1
    p1 --> p2
    p2 --> p3
```

### `numpy@2.5.2`

```mermaid
flowchart LR
    p0["pc-diagnostic"]
    p1["pyqtgraph@0.14.0"]
    p2["numpy@2.5.2"]
    p0 --> p1
    p1 --> p2
```

### `pyside6@6.11.2`

```mermaid
flowchart LR
    p0["pc-diagnostic"]
    p1["pyside6@6.11.2"]
    p0 --> p1
```

### `pyside6-addons@6.11.2`

```mermaid
flowchart LR
    p0["pc-diagnostic"]
    p1["pyside6@6.11.2"]
    p2["pyside6-addons@6.11.2"]
    p0 --> p1
    p1 --> p2
```

### `pyside6-essentials@6.11.2`

```mermaid
flowchart LR
    p0["pc-diagnostic"]
    p1["pyside6@6.11.2"]
    p2["pyside6-essentials@6.11.2"]
    p0 --> p1
    p1 --> p2
```

### `shiboken6@6.11.2`

```mermaid
flowchart LR
    p0["pc-diagnostic"]
    p1["pyside6@6.11.2"]
    p2["shiboken6@6.11.2"]
    p0 --> p1
    p1 --> p2
```

## Complete Dependency Graph

<details>
<summary>Show all 21 packages and 26 edges</summary>

```mermaid
---
config:
  layout: elk
  flowchart:
    useMaxWidth: false
    nodeSpacing: 35
    rankSpacing: 60
---
flowchart TB
    n0["cffi@2.1.1"]
    n1["colorama@0.4.6"]
    n2["cryptography@50.0.1"]
    n3["jaraco-classes@3.4.0"]
    n4["jaraco-context@6.1.2"]
    n5["jaraco-functools@4.6.0"]
    n6["jeepney@0.9.0"]
    n7["keyring@25.7.0"]
    n8["markdown-it-py@4.2.0"]
    n9["mdurl@0.1.2"]
    n10["more-itertools@11.1.0"]
    n11["numpy@2.5.2"]
    n12["pc-diagnostic"]
    n13["pycparser@3.0"]
    n14["pygments@2.21.0"]
    n15["pyqtgraph@0.14.0"]
    n16["pyside6@6.11.2"]
    n17["pyside6-addons@6.11.2"]
    n18["pyside6-essentials@6.11.2"]
    n19["rich@15.0.0"]
    n20["secretstorage@3.5.0"]
    n21["shiboken6@6.11.2"]
    n0 --> n13
    n2 --> n0
    n3 --> n10
    n5 --> n10
    n7 --> n3
    n7 --> n4
    n7 --> n5
    n7 --> n6
    n7 --> n20
    n8 --> n9
    n12 --> n7
    n12 --> n15
    n12 --> n16
    n12 --> n19
    n15 --> n1
    n15 --> n11
    n16 --> n17
    n16 --> n18
    n16 --> n21
    n17 --> n18
    n17 --> n21
    n18 --> n21
    n19 --> n8
    n19 --> n14
    n20 --> n2
    n20 --> n6
```

</details>

## Packages

| Package | Version | License | Verdict |
|---|---|---|---|
| `cffi` | `2.1.1` | `MIT-0` | `policy_violation` |
| `colorama` | `0.4.6` | `BSD-3-Clause` | `pass` |
| `cryptography` | `50.0.1` | `Apache-2.0 OR BSD-3-Clause` | `policy_violation` |
| `jaraco-classes` | `3.4.0` | `MIT` | `pass` |
| `jaraco-context` | `6.1.2` | `MIT` | `pass` |
| `jaraco-functools` | `4.6.0` | `MIT` | `pass` |
| `jeepney` | `0.9.0` | `MIT` | `pass` |
| `keyring` | `25.7.0` | `MIT` | `pass` |
| `markdown-it-py` | `4.2.0` | `MIT` | `pass` |
| `mdurl` | `0.1.2` | `MIT` | `pass` |
| `more-itertools` | `11.1.0` | `MIT` | `pass` |
| `numpy` | `2.5.2` | `BSD-3-Clause AND 0BSD AND MIT AND Zlib AND CC0-1.0` | `policy_violation` |
| `pycparser` | `3.0` | `BSD-3-Clause` | `pass` |
| `pygments` | `2.21.0` | `BSD-2-Clause` | `pass` |
| `pyqtgraph` | `0.14.0` | `MIT` | `pass` |
| `pyside6` | `6.11.2` | `LGPL-3.0-only OR GPL-2.0-only OR GPL-3.0-only` | `policy_violation` |
| `pyside6-addons` | `6.11.2` | `LGPL-3.0-only OR GPL-2.0-only OR GPL-3.0-only` | `policy_violation` |
| `pyside6-essentials` | `6.11.2` | `LGPL-3.0-only OR GPL-2.0-only OR GPL-3.0-only` | `policy_violation` |
| `rich` | `15.0.0` | `MIT` | `pass` |
| `secretstorage` | `3.5.0` | `BSD-3-Clause` | `pass` |
| `shiboken6` | `6.11.2` | `LGPL-3.0-only OR GPL-2.0-only OR GPL-3.0-only` | `policy_violation` |

## Policy Violations

| Package | License | Dependency path |
|---|---|---|
| `cffi@2.1.1` | `MIT-0` | `pc-diagnostic → keyring@25.7.0 → secretstorage@3.5.0 → cryptography@50.0.1 → cffi@2.1.1` |
| `cryptography@50.0.1` | `Apache-2.0 OR BSD-3-Clause` | `pc-diagnostic → keyring@25.7.0 → secretstorage@3.5.0 → cryptography@50.0.1` |
| `numpy@2.5.2` | `BSD-3-Clause AND 0BSD AND MIT AND Zlib AND CC0-1.0` | `pc-diagnostic → pyqtgraph@0.14.0 → numpy@2.5.2` |
| `pyside6@6.11.2` | `LGPL-3.0-only OR GPL-2.0-only OR GPL-3.0-only` | `pc-diagnostic → pyside6@6.11.2` |
| `pyside6-addons@6.11.2` | `LGPL-3.0-only OR GPL-2.0-only OR GPL-3.0-only` | `pc-diagnostic → pyside6@6.11.2 → pyside6-addons@6.11.2` |
| `pyside6-essentials@6.11.2` | `LGPL-3.0-only OR GPL-2.0-only OR GPL-3.0-only` | `pc-diagnostic → pyside6@6.11.2 → pyside6-essentials@6.11.2` |
| `shiboken6@6.11.2` | `LGPL-3.0-only OR GPL-2.0-only OR GPL-3.0-only` | `pc-diagnostic → pyside6@6.11.2 → shiboken6@6.11.2` |
