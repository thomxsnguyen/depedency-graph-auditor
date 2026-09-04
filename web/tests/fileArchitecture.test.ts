import { classifyFileArchitecture } from "../src/graph/fileArchitecture"

describe("file architecture classification", () => {
  it("detects conventional and fallback project boundaries", () => {
    expect(classifyFileArchitecture("apps/admin/src/App.tsx").project).toBe("apps/admin")
    expect(classifyFileArchitecture("packages/auth/src/token.ts").project).toBe("packages/auth")
    expect(classifyFileArchitecture("frontend/components/Button.tsx").project).toBe("frontend")
    expect(classifyFileArchitecture("engine/runtime/tasks/runner.py").project).toBe("engine")
    expect(classifyFileArchitecture("main.go").project).toBe("repository")
  })

  it("preserves project and domain casing while matching conventions case-insensitively", () => {
    expect(classifyFileArchitecture("Apps/Admin/Services/Billing/Charge.ts")).toMatchObject({
      project: "Apps/Admin",
      layer: "application",
      domain: "Billing",
    })
  })

  it("gives universal conventions precedence over language profiles", () => {
    expect(classifyFileArchitecture("frontend/api/client.test.ts")).toMatchObject({
      layer: "test",
      classification: "universal",
    })
    expect(classifyFileArchitecture("backend/config/services.py")).toMatchObject({
      layer: "configuration",
      classification: "universal",
    })
  })

  it("uses JavaScript and TypeScript conventions", () => {
    expect(classifyFileArchitecture("frontend/components/account/Card.tsx")).toMatchObject({ layer: "presentation", domain: "account" })
    expect(classifyFileArchitecture("frontend/api/users.ts").layer).toBe("infrastructure")
    expect(classifyFileArchitecture("backend/api/users.ts").layer).toBe("transport")
    expect(classifyFileArchitecture("frontend/state/session.ts").layer).toBe("application")
  })

  it("uses Python conventions", () => {
    expect(classifyFileArchitecture("backend/services/diagnostics/cpu.py")).toMatchObject({ layer: "application", domain: "diagnostics" })
    expect(classifyFileArchitecture("pc_diagnostic/providers/windows/cpu.py")).toMatchObject({ layer: "infrastructure", domain: "windows" })
    expect(classifyFileArchitecture("backend/schemas/user.py").layer).toBe("domain")
    expect(classifyFileArchitecture("backend/main.py")).toMatchObject({ layer: "entrypoint", domain: "General" })
  })

  it("uses Go conventions without treating internal as a role", () => {
    expect(classifyFileArchitecture("cmd/server/main.go").layer).toBe("entrypoint")
    expect(classifyFileArchitecture("internal/repository/postgres/users.go")).toMatchObject({ layer: "persistence", domain: "postgres" })
    expect(classifyFileArchitecture("internal/work/runner.go")).toMatchObject({ layer: "other", domain: "work" })
    expect(classifyFileArchitecture("pkg/token/token.go").layer).toBe("shared")
  })

  it("preserves unknown directory structure as the fallback domain", () => {
    expect(classifyFileArchitecture("engine/runtime/tasks/runner.py")).toMatchObject({
      project: "engine",
      layer: "other",
      domain: "runtime",
      classification: "fallback",
      confidence: "fallback",
    })
    expect(classifyFileArchitecture("custom.py")).toMatchObject({
      project: "repository",
      layer: "other",
      domain: "General",
    })
  })

  it("classifies a polyglot set independently per file", () => {
    const results = [
      "frontend/components/App.tsx",
      "backend/services/users.py",
      "internal/storage/users.go",
    ].map(classifyFileArchitecture)
    expect(results.map((result) => result.layer)).toEqual(["presentation", "application", "persistence"])
  })
})
