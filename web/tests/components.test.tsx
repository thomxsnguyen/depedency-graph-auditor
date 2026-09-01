import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { AuditSidebar } from "../src/components/AuditSidebar/AuditSidebar"
import { NodeInspector } from "../src/components/NodeInspector/NodeInspector"
import { QueueStrip } from "../src/components/QueueStrip/QueueStrip"
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
        filters={{ search: "", violationsOnly: false, directOnly: false }}
        open
        onClose={vi.fn()}
        onSearch={onSearch}
        onFilter={onFilter}
      />,
    )

    await user.type(screen.getByRole("searchbox"), "vite")
    await user.click(screen.getByRole("checkbox", { name: "Violations only" }))
    expect(onSearch).toHaveBeenCalled()
    expect(onFilter).toHaveBeenCalledWith("violationsOnly", true)
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
})
