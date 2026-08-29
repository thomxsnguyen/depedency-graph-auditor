# Dependency Audit Report

## Summary

- Root: `https://github.com/trevorngo24/paper-trading-platform/package.json`
- Packages scanned: 4
- Policy violations: 0

## Dependency Overview

```mermaid
flowchart TB
    n0["https://github.com/trevorngo24/paper-trading-platform/package.json"]
    n1["react-router-dom@7.18.3"]
    n0 --> n1
```

## Violation Paths

No policy violation paths.

## Complete Dependency Graph

<details>
<summary>Show all 4 packages and 4 edges</summary>

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
    n0["cookie@1.1.1"]
    n1["https://github.com/trevorngo24/paper-trading-platform/package.json"]
    n2["react-router@7.18.3"]
    n3["react-router-dom@7.18.3"]
    n4["set-cookie-parser@2.7.2"]
    n1 --> n3
    n2 --> n0
    n2 --> n4
    n3 --> n2
```

</details>

## Packages

| Package | Version | License | Verdict |
|---|---|---|---|
| `cookie` | `1.1.1` | `MIT` | `pass` |
| `react-router` | `7.18.3` | `MIT` | `pass` |
| `react-router-dom` | `7.18.3` | `MIT` | `pass` |
| `set-cookie-parser` | `2.7.2` | `MIT` | `pass` |

## Policy Violations

| Package | License | Dependency path |
|---|---|---|
