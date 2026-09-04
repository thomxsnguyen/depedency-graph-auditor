import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { OperationsDashboard } from "../src/app/OperationsDashboard"
import { JobApi } from "../src/data/JobApi"

vi.mock("../src/data/JobApi", () => ({
  JobApi: {
    list: vi.fn(), get: vi.fn(), submit: vi.fn(), cancel: vi.fn(), retry: vi.fn(),
    listDLQ: vi.fn(), replay: vi.fn(),
  },
}))

const row = {
  id: "12345678-abcd", type: "demo", payload: { durationMs: 0 }, status: "completed" as const,
  attempts: 1, maxAttempts: 5, scheduledAt: "2026-09-04T12:00:00Z",
  createdAt: "2026-09-04T12:00:00Z", completedAt: "2026-09-04T12:00:01Z",
}

beforeEach(() => {
  vi.mocked(JobApi.list).mockResolvedValue({ jobs: [row], counts: { completed: 1, running: 0 } })
  vi.mocked(JobApi.listDLQ).mockResolvedValue({ entries: [] })
  vi.mocked(JobApi.get).mockResolvedValue({
    job: row, attempts: [{ attempt: 1, workerId: "worker-1", status: "completed", startedAt: row.createdAt }],
    events: [{ id: 1, type: "submitted", occurredAt: row.createdAt }, { id: 2, type: "completed", workerId: "worker-1", occurredAt: row.completedAt }],
    result: { message: "done" },
  })
})

afterEach(() => vi.clearAllMocks())

it("shows queue totals and a selected job lifecycle", async () => {
  const user = userEvent.setup()
  render(<OperationsDashboard />)
  expect(await screen.findByText("12345678")).toBeInTheDocument()
  expect(screen.getByRole("region", { name: "Queue totals" })).toHaveTextContent("completed1")
  await user.click(screen.getByText("12345678"))
  await waitFor(() => expect(screen.getByText("Lifecycle")).toBeInTheDocument())
  expect(screen.getAllByText(/worker-1/)).toHaveLength(2)
  expect(screen.getByText(/"message": "done"/)).toBeInTheDocument()
})

it("submits the controlled retry demonstration", async () => {
  const user = userEvent.setup()
  vi.mocked(JobApi.submit).mockResolvedValue({ job: row })
  render(<OperationsDashboard />)
  await screen.findByText("12345678")
  await user.click(screen.getByRole("button", { name: /retry twice/i }))
  await waitFor(() => expect(JobApi.submit).toHaveBeenCalledWith("demo", { durationMs: 0, transientFailures: 2 }))
})
