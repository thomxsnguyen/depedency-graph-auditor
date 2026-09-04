import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { ReactFlowProvider } from "@xyflow/react"
import { AuditSidebar } from "../src/components/AuditSidebar/AuditSidebar"
import { NodeInspector } from "../src/components/NodeInspector/NodeInspector"
import { QueueStrip } from "../src/components/QueueStrip/QueueStrip"
import { FileInspector } from "../src/components/FileInspector/FileInspector"
import { FileSidebar } from "../src/components/FileSidebar/FileSidebar"
import { GraphModeSelector } from "../src/components/GraphModeSelector/GraphModeSelector"
import { GitHubFileGraphForm } from "../src/components/FileSidebar/GitHubFileGraphForm"
import { FileNode } from "../src/components/FileGraphCanvas/FileNode"
import { ArchitectureNode } from "../src/components/FileGraphCanvas/ArchitectureNode"
import { DomainNode } from "../src/components/FileGraphCanvas/DomainNode"
import type { AuditPackage } from "../src/types/audit"

const counts = { pending: 0, running: 0, retrying: 1, completed: 10, dead_lettered: 1 }

describe("Classic page components", () => {
  it("shows a file node category independently from diagnostics", () => {
    render(
      <ReactFlowProvider>
        <FileNode
          id="file:frontend%2FApp.tsx"
          type="file"
          data={{
            entityKind: "file",
            path: "frontend/App.tsx",
            fileName: "App.tsx",
            parentPath: "frontend",
            category: "frontend",
            project: "frontend",
            layer: "entrypoint",
            domain: "General",
            diagnosticCount: 1,
            selected: false,
            searchMatch: true,
            rank: 0,
            lane: "main",
          }}
          selected={false}
          dragging={false}
          draggable
          selectable
          deletable={false}
          isConnectable={false}
          zIndex={0}
          parentId={undefined}
          positionAbsoluteX={0}
          positionAbsoluteY={0}
        />
      </ReactFlowProvider>,
    )
    expect(screen.getByText("Frontend")).toBeInTheDocument()
    expect(screen.getByLabelText("1 diagnostics")).toBeInTheDocument()
  })

  it("shows architecture counts and expands the requested layer", async () => {
    const user = userEvent.setup()
    const onToggle = vi.fn()
    render(
      <ReactFlowProvider>
        <ArchitectureNode
          id="architecture:frontend:presentation"
          type="architecture"
          data={{
            entityKind: "architecture",
            id: "architecture:frontend:presentation",
            project: "frontend",
            layer: "presentation",
            layerLabel: "Presentation",
            label: "Presentation",
            domainCount: 3,
            expanded: false,
            fileCount: 12,
            internalDependencyCount: 18,
            diagnosticCount: 2,
            selected: false,
            searchMatch: true,
            rank: 1,
            lane: "main",
            onToggle,
          }}
          selected={false}
          dragging={false}
          draggable
          selectable
          deletable={false}
          isConnectable={false}
          zIndex={0}
          parentId={undefined}
          positionAbsoluteX={0}
          positionAbsoluteY={0}
        />
      </ReactFlowProvider>,
    )
    expect(screen.getByText("3 folders · 12 files")).toBeInTheDocument()
    expect(screen.getByLabelText("2 diagnostics")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Expand Presentation in frontend" }))
    expect(onToggle).toHaveBeenCalledWith("architecture:frontend:presentation", "frontend")
  })

  it("shows a domain and expands it to files", async () => {
    const user = userEvent.setup()
    const onToggle = vi.fn()
    render(
      <ReactFlowProvider>
        <DomainNode
          id="domain:backend:application:diagnostics"
          type="domain"
          data={{
            entityKind: "domain",
            id: "domain:backend:application:diagnostics",
            architectureId: "architecture:backend:application",
            project: "backend",
            layer: "application",
            layerLabel: "Services",
            domain: "diagnostics",
            expanded: false,
            fileCount: 4,
            internalDependencyCount: 3,
            diagnosticCount: 0,
            selected: false,
            searchMatch: true,
            rank: 3,
            lane: "main",
            onToggle,
          }}
          selected={false}
          dragging={false}
          draggable
          selectable
          deletable={false}
          isConnectable={false}
          zIndex={0}
          parentId={undefined}
          positionAbsoluteX={0}
          positionAbsoluteY={0}
        />
      </ReactFlowProvider>,
    )
    await user.click(screen.getByRole("button", { name: "Expand folder diagnostics in Services" }))
    expect(onToggle).toHaveBeenCalledWith(
      "domain:backend:application:diagnostics",
      "architecture:backend:application",
    )
  })

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
        repositorySubmitting={false}
        repositoryError={null}
        onClose={vi.fn()}
        onSearch={onSearch}
        onRepositorySubmit={vi.fn()}
        onRepositoryChange={vi.fn()}
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
        repositorySubmitting={false}
        repositoryError={null}
        onClose={vi.fn()}
        onSearch={onSearch}
        onRepositorySubmit={vi.fn()}
        onRepositoryChange={vi.fn()}
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

  it("validates and submits a public GitHub repository", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(
      <GitHubFileGraphForm
        submitting={false}
        error={null}
        onSubmit={onSubmit}
        onChange={vi.fn()}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Analyze repository" }))
    expect(screen.getByRole("alert")).toHaveTextContent("Enter a GitHub repository URL.")
    await user.type(screen.getByLabelText("GitHub repository URL"), "https://github.com/owner/repo")
    await user.type(screen.getByLabelText(/Ref/), "main")
    await user.click(screen.getByRole("button", { name: "Analyze repository" }))
    expect(onSubmit).toHaveBeenCalledWith({
      repositoryUrl: "https://github.com/owner/repo",
      ref: "main",
    })
  })

  it("disables repository inputs while analysis is running", () => {
    render(
      <GitHubFileGraphForm
        submitting
        error={null}
        onSubmit={vi.fn()}
        onChange={vi.fn()}
      />,
    )
    expect(screen.getByLabelText("GitHub repository URL")).toBeDisabled()
    expect(screen.getByLabelText(/Ref/)).toBeDisabled()
    expect(screen.getByRole("button", { name: "Analyzing repository…" })).toBeDisabled()
  })
})
