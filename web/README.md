# Dependency Audit Studio UI

Local, fixture-backed frontend for the dependency auditor showcase. It does not
call a backend or modify dependency manifests. The default graph is a direct-
dependency overview; enable **Entire dependency graph** in the filters to show
all 185 audited packages and 275 edges from the complete demo report.

Use **Add presentation box** in the graph toolbar to place sharp text boxes.
Drag between box handles to add presentation-only arrows. These elements are
saved locally and never alter audited dependencies.

```bash
npm install
npm run dev
```

Validation:

```bash
npm run typecheck
npm run lint
npm run test
npm run build
```
