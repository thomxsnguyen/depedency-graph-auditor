import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { AuditSidebar } from "../src/components/AuditSidebar/AuditSidebar"
import { NodeInspector } from "../src/components/NodeInspector/NodeInspector"
import { QueueStrip } from "../src/components/QueueStrip/QueueStrip"
import { FileInspector } from "../src/components/FileInspector/FileInspector"
import { FileSidebar } from "../src/components/FileSidebar/FileSidebar"
import { GraphModeSelector } from "../src/components/GraphModeSelector/GraphModeSelector"
import type { AuditPackage } from "../src/types/audit"

const counts = { pending: 0, running: 0, retrying: 1, completed: 10, dead_lettered: 1 }

describe("Classic page components", () => {
  it("changes search and filter controls", async () => {
    const user = userEvent.setup()
    const onSearch = vi.fn()
    const onFilter = vi.fn()
    render(
      <AuditSidebar
        packageCount={12}
        violationCount={1}
        counts={counts}
        filters={{ search: "", violationsOnly: false, completeGraph: false }}
        open
        onClose={vi.fn()}
        onSearch={onSearch}
        onFilter={onFilter}
      />,
    )

    await user.type(screen.getByRole("searchbox"), "vite")
    await user.click(screen.getByRole("checkbox", { name: "Violations only" }))
    await user.click(screen.getByRole("checkbox", { name: "Entire dependency graph" }))
    expect(onSearch).toHaveBeenCalled()
    expect(onFilter).toHaveBeenCalledWith("violationsOnly", true)
    expect(onFilter).toHaveBeenCalledWith("completeGraph", true)
  })

  it("saves a presentation note from the inspector", async () => {
    const user = userEvent.setup()
    const onAnnotate = vi.fn()
    const packageRow: AuditPackage = {
      id: "vite@7.1.2",
      name: "vite",
      version: "7.1.2",
      license: "MIT",
      verdict: "pass",
      status: "completed",
      attempts: 1,
    }
    render(
      <NodeInspector
        root="personal-portfolio"
        packageRow={packageRow}
        path={["root", packageRow.id]}
        edges={[{ id: "edge", from: "root", to: packageRow.id }]}
        annotation=""
        open
        saveStatus="idle"
        onClose={vi.fn()}
        onAnnotate={onAnnotate}
      />,
    )

    await user.type(screen.getByLabelText("Presentation note"), "Pinned for review")
    await user.click(screen.getByRole("button", { name: "Save note" }))
    expect(onAnnotate).toHaveBeenCalledWith("Pinned for review")
  })

  it("reveals recent activity without replacing queue counts", async () => {
    const user = userEvent.setup()
    const onToggle = vi.fn()
    render(<QueueStrip counts={counts} events={[]} open={false} onToggle={onToggle} />)
    expect(screen.getByText("10")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: /recent activity/i }))
    expect(onToggle).toHaveBeenCalledOnce()
  })

  it("switches graph mode with the segmented control", async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<GraphModeSelector mode="dependencies" onChange={onChange} />)
    await user.click(screen.getByRole("tab", { name: "Files" }))
    expect(onChange).toHaveBeenCalledWith("files")
    screen.getByRole("tab", { name: "Dependencies" }).focus()
    await user.keyboard("{ArrowRight}")
    expect(onChange).toHaveBeenLastCalledWith("files")
  })

  it("searches files and clears the query", async () => {
    const user = userEvent.setup()
    const onSearch = vi.fn()
    const { rerender } = render(
      <FileSidebar
        fileCount={9}
        importCount={6}
        diagnosticCount={1}
        search=""
        open
        onClose={vi.fn()}
        onSearch={onSearch}
      />,
    )
    await user.type(screen.getByRole("searchbox"), "App.tsx")
    expect(onSearch).toHaveBeenCalled()
    rerender(
      <FileSidebar
        fileCount={9}
        importCount={6}
        diagnosticCount={1}
        search="App.tsx"
        open
        onClose={vi.fn()}
        onSearch={onSearch}
      />,
    )
    await user.click(screen.getByRole("button", { name: "Clear file search" }))
    expect(onSearch).toHaveBeenLastCalledWith("")
  })

  it("shows selected file relationships and diagnostics", () => {
    render(
      <FileInspector
        file={{ path: "frontend/App.tsx" }}
        incoming={["frontend/main.tsx"]}
        outgoing={["frontend/Button.tsx"]}
        diagnostics={[{ path: "frontend/App.tsx", import: "./missing", message: "unresolved local import" }]}
        open
        onClose={vi.fn()}
      />,
    )
    expect(screen.getByRole("heading", { name: "App.tsx" })).toBeInTheDocument()
    expect(screen.getByText("frontend/Button.tsx")).toBeInTheDocument()
    expect(screen.getByText("frontend/main.tsx")).toBeInTheDocument()
    expect(screen.getByText("unresolved local import")).toBeInTheDocument()
  })
})
