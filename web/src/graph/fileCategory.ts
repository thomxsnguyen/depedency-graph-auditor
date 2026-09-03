export type FileCategory =
  | "application"
  | "frontend"
  | "configuration"
  | "test"
  | "script"
  | "generated"
  | "general"

export const FILE_CATEGORY_DETAILS: ReadonlyArray<{
  category: FileCategory
  label: string
  color: string
}> = [
  { category: "application", label: "Application", color: "#64707d" },
  { category: "frontend", label: "Frontend", color: "#758195" },
  { category: "configuration", label: "Configuration", color: "#8a8178" },
  { category: "test", label: "Tests", color: "#748477" },
  { category: "script", label: "Scripts & tools", color: "#887c70" },
  { category: "generated", label: "Generated & vendor", color: "#9ca3af" },
  { category: "general", label: "General", color: "#6b7280" },
]

const GENERATED_DIRECTORIES = new Set(["dist", "build", "coverage", "generated", "gen", "vendor", "node_modules"])
const TEST_DIRECTORIES = new Set(["test", "tests", "__tests__", "spec", "specs"])
const CONFIG_DIRECTORIES = new Set(["config", "configs", "configuration", ".github"])
const FRONTEND_DIRECTORIES = new Set(["frontend", "client", "web", "ui", "components", "pages", "views", "hooks"])
const SCRIPT_DIRECTORIES = new Set(["script", "scripts", "tool", "tools", "bin"])
const APPLICATION_DIRECTORIES = new Set(["src", "app", "application", "backend", "server", "cmd", "internal", "lib", "pkg"])

function hasDirectory(parts: readonly string[], directories: ReadonlySet<string>): boolean {
  return parts.slice(0, -1).some((part) => directories.has(part))
}

export function classifyFile(path: string): FileCategory {
  const normalized = path.replaceAll("\\", "/").toLowerCase()
  const parts = normalized.split("/").filter(Boolean)
  const fileName = parts.at(-1) ?? normalized

  if (hasDirectory(parts, GENERATED_DIRECTORIES) || /(?:\.generated|\.gen)\.[^.]+$/.test(fileName)) {
    return "generated"
  }
  if (
    hasDirectory(parts, TEST_DIRECTORIES)
    || /(?:^test_.+|_test|\.(?:test|spec))\.[^.]+$/.test(fileName)
    || fileName === "conftest.py"
  ) {
    return "test"
  }
  if (
    hasDirectory(parts, CONFIG_DIRECTORIES)
    || /(?:^|\.)config\.[^.]+$/.test(fileName)
    || /^(?:settings|configuration)\.[^.]+$/.test(fileName)
  ) {
    return "configuration"
  }
  if (hasDirectory(parts, FRONTEND_DIRECTORIES)) return "frontend"
  if (hasDirectory(parts, SCRIPT_DIRECTORIES)) return "script"
  if (hasDirectory(parts, APPLICATION_DIRECTORIES)) return "application"
  return "general"
}

export function categoryDetails(category: FileCategory) {
  return FILE_CATEGORY_DETAILS.find((details) => details.category === category)!
}
