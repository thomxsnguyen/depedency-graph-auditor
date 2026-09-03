import { HttpRepositoryFileGraphDataSource } from "../src/data/RepositoryFileGraphDataSource"

const validGraph = {
  schemaVersion: 1,
  root: "repo",
  nodes: [{ path: "app.py" }],
  edges: [],
  diagnostics: [],
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("repository file graph data source", () => {
  it("posts a trimmed repository request and validates the response", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue(validGraph),
    })
    vi.stubGlobal("fetch", fetchMock)

    const result = await new HttpRepositoryFileGraphDataSource().analyze({
      repositoryUrl: " https://github.com/owner/repo ",
      ref: " main ",
    })

    expect(result).toEqual(validGraph)
    expect(fetchMock).toHaveBeenCalledWith("/api/file-graphs", expect.objectContaining({
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ repositoryUrl: "https://github.com/owner/repo", ref: "main" }),
    }))
  })

  it("omits a blank optional ref", async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue(validGraph) })
    vi.stubGlobal("fetch", fetchMock)
    await new HttpRepositoryFileGraphDataSource().analyze({
      repositoryUrl: "https://github.com/owner/repo",
      ref: "",
    })
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      repositoryUrl: "https://github.com/owner/repo",
    })
  })

  it("uses safe server errors and rejects invalid success payloads", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      json: vi.fn().mockResolvedValue({ error: "GitHub rate limited the request. Try again later." }),
    }))
    await expect(new HttpRepositoryFileGraphDataSource().analyze({
      repositoryUrl: "https://github.com/owner/repo",
    })).rejects.toThrow("GitHub rate limited")

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ schemaVersion: 2 }),
    }))
    await expect(new HttpRepositoryFileGraphDataSource().analyze({
      repositoryUrl: "https://github.com/owner/repo",
    })).rejects.toThrow("Unsupported file graph schema")
  })

  it("returns a bounded network error", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("connection details")))
    await expect(new HttpRepositoryFileGraphDataSource().analyze({
      repositoryUrl: "https://github.com/owner/repo",
    })).rejects.toThrow("The file graph service could not be reached.")
  })
})
