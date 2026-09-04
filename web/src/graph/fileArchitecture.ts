export type ArchitectureLayer =
  | "entrypoint"
  | "presentation"
  | "transport"
  | "application"
  | "domain"
  | "persistence"
  | "infrastructure"
  | "shared"
  | "configuration"
  | "test"
  | "tooling"
  | "other"

export type ArchitectureClassification = "universal" | "language-convention" | "generic" | "fallback"
export type ArchitectureLane = "main" | "configuration" | "shared" | "test" | "tooling"

export interface FileArchitecture {
  filePath: string
  project: string
  layer: ArchitectureLayer
  domain: string
  classification: ArchitectureClassification
  confidence: "inferred" | "fallback"
  rank: number
  lane: ArchitectureLane
}

export const ARCHITECTURE_LAYER_LABELS: Record<ArchitectureLayer, string> = {
  entrypoint: "Entrypoints",
  presentation: "Presentation",
  transport: "Transport / API",
  application: "Services",
  domain: "Domain",
  persistence: "Persistence",
  infrastructure: "Infrastructure",
  shared: "Shared",
  configuration: "Configuration",
  test: "Tests",
  tooling: "Tooling",
  other: "Other",
}

const PROJECT_DIRECTORIES = new Set(["frontend", "backend", "client", "server", "web", "api", "cli"])
const MULTI_PROJECT_DIRECTORIES = new Set(["apps", "packages", "services"])
const TEST_DIRECTORIES = new Set(["test", "tests", "__tests__", "spec", "specs", "testdata"])
const CONFIG_DIRECTORIES = new Set(["config", "configs", "configuration", ".github"])
const TOOLING_DIRECTORIES = new Set(["script", "scripts", "tool", "tools", "bin"])

interface Match {
  layer: ArchitectureLayer
  matchedIndex: number
  classification: Exclude<ArchitectureClassification, "fallback">
}

interface ProjectBoundary {
  project: string
  depth: number
}

function normalizedParts(path: string): string[] {
  return path.replaceAll("\\", "/").split("/").filter(Boolean)
}

function projectBoundary(parts: readonly string[], comparableParts: readonly string[]): ProjectBoundary {
  const directories = parts.slice(0, -1)
  const comparableDirectories = comparableParts.slice(0, -1)
  if (directories.length >= 2 && MULTI_PROJECT_DIRECTORIES.has(comparableDirectories[0])) {
    return { project: `${directories[0]}/${directories[1]}`, depth: 2 }
  }
  if (directories.length > 0 && PROJECT_DIRECTORIES.has(comparableDirectories[0])) {
    return { project: directories[0], depth: 1 }
  }
  if (directories.length > 0) return { project: directories[0], depth: 1 }
  return { project: "repository", depth: 0 }
}

function segmentMatch(parts: readonly string[], values: ReadonlySet<string>): number {
  return parts.slice(0, -1).findIndex((part) => values.has(part))
}

function matchLayer(
  parts: readonly string[],
  classification: Match["classification"],
  rules: readonly [ArchitectureLayer, ReadonlySet<string>][],
): Match | null {
  for (const [layer, values] of rules) {
    const matchedIndex = segmentMatch(parts, values)
    if (matchedIndex >= 0) return { layer, matchedIndex, classification }
  }
  return null
}

function universalMatch(parts: readonly string[]): Match | null {
  const fileName = parts.at(-1) ?? ""
  const testIndex = segmentMatch(parts, TEST_DIRECTORIES)
  if (testIndex >= 0 || /(?:^test_.+|_test|\.(?:test|spec))\.[^.]+$/.test(fileName)) {
    return { layer: "test", matchedIndex: testIndex >= 0 ? testIndex : parts.length - 1, classification: "universal" }
  }
  const configIndex = segmentMatch(parts, CONFIG_DIRECTORIES)
  if (configIndex >= 0 || /(?:^|\.)config\.[^.]+$/.test(fileName) || /^settings\.[^.]+$/.test(fileName)) {
    return { layer: "configuration", matchedIndex: configIndex >= 0 ? configIndex : parts.length - 1, classification: "universal" }
  }
  const toolingIndex = segmentMatch(parts, TOOLING_DIRECTORIES)
  if (toolingIndex >= 0) return { layer: "tooling", matchedIndex: toolingIndex, classification: "universal" }
  return null
}

function entrypointMatch(
  parts: readonly string[],
  names: RegExp,
  classification: Match["classification"],
): Match | null {
  const fileName = parts.at(-1) ?? ""
  if (!names.test(fileName)) return null
  return { layer: "entrypoint", matchedIndex: parts.length - 1, classification }
}

function javascriptTypeScriptMatch(parts: readonly string[], project: string): Match | null {
  const match = matchLayer(parts, "language-convention", [
    ["transport", new Set(["routes", "controllers", "middleware"])],
    ["presentation", new Set(["pages", "screens", "views", "components", "ui", "hooks"])],
    ["domain", new Set(["models", "entities", "domain"])],
    ["application", new Set(["services", "use-cases", "usecases", "store", "state", "reducers"])],
    ["infrastructure", new Set(project === "frontend" || project === "client" ? ["api", "clients"] : ["clients"])],
    ["shared", new Set(["utils", "lib", "shared"])],
  ])
  return match ?? entrypointMatch(parts, /^(?:main|index|app|server)\.[^.]+$/, "language-convention")
}

function pythonMatch(parts: readonly string[]): Match | null {
  const match = matchLayer(parts, "language-convention", [
    ["persistence", new Set(["repositories", "db", "database", "migrations"])],
    ["transport", new Set(["api", "routes", "endpoints"])],
    ["presentation", new Set(["views", "templates", "gui", "widgets"])],
    ["domain", new Set(["models", "entities", "domain", "schemas", "serializers"])],
    ["application", new Set(["services", "use_cases", "usecases"])],
    ["infrastructure", new Set(["providers", "adapters", "integrations"])],
    ["shared", new Set(["utils", "helpers", "common"])],
  ])
  return match ?? entrypointMatch(parts, /^(?:main|__main__|manage)\.py$/, "language-convention")
}

function goMatch(parts: readonly string[]): Match | null {
  const fileName = parts.at(-1) ?? ""
  const cmdIndex = parts.slice(0, -1).indexOf("cmd")
  if (cmdIndex >= 0 || fileName === "main.go") {
    return { layer: "entrypoint", matchedIndex: cmdIndex >= 0 ? cmdIndex : parts.length - 1, classification: "language-convention" }
  }
  return matchLayer(parts, "language-convention", [
    ["configuration", new Set(["config"])],
    ["persistence", new Set(["repository", "storage", "database"])],
    ["transport", new Set(["handlers", "http", "api"])],
    ["domain", new Set(["domain", "model", "models", "entity", "entities"])],
    ["application", new Set(["service", "services", "usecase", "usecases"])],
    ["infrastructure", new Set(["adapter", "adapters", "client", "clients"])],
    ["shared", new Set(["pkg"])],
  ])
}

function languageMatch(parts: readonly string[], project: string): Match | null {
  const fileName = parts.at(-1) ?? ""
  const extension = fileName.slice(fileName.lastIndexOf("."))
  if ([".js", ".jsx", ".ts", ".tsx"].includes(extension)) return javascriptTypeScriptMatch(parts, project)
  if (extension === ".py") return pythonMatch(parts)
  if (extension === ".go") return goMatch(parts)
  return null
}

function genericMatch(parts: readonly string[]): Match | null {
  const match = matchLayer(parts, "generic", [
    ["persistence", new Set(["repository", "repositories", "persistence", "database", "db", "storage", "dao"])],
    ["transport", new Set(["api", "route", "routes", "controller", "controllers", "handler", "handlers", "transport"])],
    ["presentation", new Set(["ui", "component", "components", "page", "pages", "view", "views", "screen", "screens"])],
    ["domain", new Set(["domain", "model", "models", "entity", "entities", "aggregate", "aggregates"])],
    ["application", new Set(["service", "services", "usecase", "usecases", "application"])],
    ["infrastructure", new Set(["adapter", "adapters", "provider", "providers", "integration", "integrations", "client", "clients"])],
    ["shared", new Set(["shared", "common", "util", "utils", "helper", "helpers", "lib"])],
  ])
  if (match) return match
  const cmdIndex = parts.slice(0, -1).indexOf("cmd")
  if (cmdIndex >= 0) return { layer: "entrypoint", matchedIndex: cmdIndex, classification: "generic" }
  return entrypointMatch(parts, /^(?:main|index|app|server)\.[^.]+$/, "generic")
}

function layerRank(layer: ArchitectureLayer): number {
  if (layer === "entrypoint" || layer === "configuration" || layer === "tooling") return 0
  if (layer === "presentation" || layer === "test") return 1
  if (layer === "transport") return 2
  if (layer === "application" || layer === "shared" || layer === "other") return 3
  if (layer === "domain") return 4
  return 5
}

function layerLane(layer: ArchitectureLayer): ArchitectureLane {
  if (layer === "configuration") return "configuration"
  if (layer === "shared") return "shared"
  if (layer === "test") return "test"
  if (layer === "tooling") return "tooling"
  return "main"
}

export function classifyFileArchitecture(path: string): FileArchitecture {
  const parts = normalizedParts(path)
  const comparableParts = parts.map((part) => part.toLowerCase())
  const boundary = projectBoundary(parts, comparableParts)
  const match = universalMatch(comparableParts)
    ?? languageMatch(comparableParts, boundary.project.toLowerCase())
    ?? genericMatch(comparableParts)
  const layer = match?.layer ?? "other"
  const directories = parts.slice(0, -1)
  const fixedGeneralLayers = new Set<ArchitectureLayer>(["entrypoint", "configuration", "test", "tooling", "shared"])
  let domain = "General"
  if (layer === "other") {
    domain = directories[boundary.depth] ?? "General"
  } else if (!fixedGeneralLayers.has(layer) && match) {
    domain = directories[match.matchedIndex + 1] ?? "General"
  }

  return {
    filePath: path.replaceAll("\\", "/"),
    project: boundary.project,
    layer,
    domain,
    classification: match?.classification ?? "fallback",
    confidence: match ? "inferred" : "fallback",
    rank: layerRank(layer),
    lane: layerLane(layer),
  }
}
