# Dependency Audit Report

## Summary

- Root: `paper-trading-platform`
- Python platform: `linux`
- Python version: `3.12`
- Packages scanned: 17
- Policy violations: 2

## Dependency Overview

```mermaid
flowchart TB
    n0["annotated-doc@0.0.4"]
    n1["annotated-types@0.7.0"]
    n2["anyio@4.12.1"]
    n3["click@8.3.1"]
    n4["h11@0.16.0"]
    n5["idna@3.11"]
    n6["paper-trading-platform"]
    n7["pydantic@2.12.5"]
    n8["starlette@0.52.1"]
    n9["typing-extensions@4.15.0"]
    n10["typing-inspection@0.4.2"]
    n11["uvicorn@0.40.0"]
    n6 --> n0
    n6 --> n1
    n6 --> n2
    n6 --> n3
    n6 --> n4
    n6 --> n5
    n6 --> n7
    n6 --> n8
    n6 --> n9
    n6 --> n10
    n6 --> n11
```

## Violation Paths

### `typing-extensions@4.15.0`

```mermaid
flowchart LR
    p0["paper-trading-platform"]
    p1["typing-extensions@4.15.0"]
    p0 --> p1
```

### `typing-extensions@4.16.0`

```mermaid
flowchart LR
    p0["paper-trading-platform"]
    p1["pydantic@2.12.5"]
    p2["typing-extensions@4.16.0"]
    p0 --> p1
    p1 --> p2
```

## Complete Dependency Graph

<details>
<summary>Show all 17 packages and 24 edges</summary>

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
    n0["annotated-doc@0.0.4"]
    n1["annotated-types@0.7.0"]
    n2["annotated-types@0.8.0"]
    n3["anyio@4.12.1"]
    n4["anyio@4.14.2"]
    n5["click@8.3.1"]
    n6["click@8.5.0"]
    n7["h11@0.16.0"]
    n8["idna@3.11"]
    n9["idna@3.19"]
    n10["paper-trading-platform"]
    n11["pydantic@2.12.5"]
    n12["starlette@0.52.1"]
    n13["typing-extensions@4.15.0"]
    n14["typing-extensions@4.16.0"]
    n15["typing-inspection@0.4.2"]
    n16["typing-inspection@0.4.4"]
    n17["uvicorn@0.40.0"]
    n3 --> n9
    n3 --> n14
    n4 --> n9
    n4 --> n14
    n10 --> n0
    n10 --> n1
    n10 --> n3
    n10 --> n5
    n10 --> n7
    n10 --> n8
    n10 --> n11
    n10 --> n12
    n10 --> n13
    n10 --> n15
    n10 --> n17
    n11 --> n2
    n11 --> n14
    n11 --> n16
    n12 --> n4
    n12 --> n14
    n15 --> n14
    n16 --> n14
    n17 --> n6
    n17 --> n7
```

</details>

## Packages

| Package | Version | License | Verdict |
|---|---|---|---|
| `annotated-doc` | `0.0.4` | `MIT` | `pass` |
| `annotated-types` | `0.7.0` | `MIT` | `pass` |
| `annotated-types` | `0.8.0` | `MIT` | `pass` |
| `anyio` | `4.12.1` | `MIT` | `pass` |
| `anyio` | `4.14.2` | `MIT` | `pass` |
| `click` | `8.3.1` | `BSD-3-Clause` | `pass` |
| `click` | `8.5.0` | `BSD-3-Clause` | `pass` |
| `h11` | `0.16.0` | `MIT` | `pass` |
| `idna` | `3.11` | `BSD-3-Clause` | `pass` |
| `idna` | `3.19` | `BSD-3-Clause` | `pass` |
| `pydantic` | `2.12.5` | `MIT` | `pass` |
| `starlette` | `0.52.1` | `BSD-3-Clause` | `pass` |
| `typing-extensions` | `4.15.0` | `PSF-2.0` | `policy_violation` |
| `typing-extensions` | `4.16.0` | `PSF-2.0` | `policy_violation` |
| `typing-inspection` | `0.4.2` | `MIT` | `pass` |
| `typing-inspection` | `0.4.4` | `MIT` | `pass` |
| `uvicorn` | `0.40.0` | `BSD-3-Clause` | `pass` |

## Policy Violations

| Package | License | Dependency path |
|---|---|---|
| `typing-extensions@4.15.0` | `PSF-2.0` | `paper-trading-platform → typing-extensions@4.15.0` |
| `typing-extensions@4.16.0` | `PSF-2.0` | `paper-trading-platform → pydantic@2.12.5 → typing-extensions@4.16.0` |
