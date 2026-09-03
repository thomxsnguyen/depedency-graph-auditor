import { parseFileGraph } from "./FixtureFileGraphDataSource"
import type { FileGraphSnapshot } from "../types/fileGraph"

export interface GitHubFileGraphRequest {
  repositoryUrl: string
  ref?: string
}

export interface RepositoryFileGraphDataSource {
  analyze(request: GitHubFileGraphRequest, signal?: AbortSignal): Promise<FileGraphSnapshot>
}

interface ErrorResponse {
  error?: unknown
}

export class HttpRepositoryFileGraphDataSource implements RepositoryFileGraphDataSource {
  async analyze(request: GitHubFileGraphRequest, signal?: AbortSignal): Promise<FileGraphSnapshot> {
    const repositoryUrl = request.repositoryUrl.trim()
    const ref = request.ref?.trim()
    let response: Response
    try {
      response = await fetch("/api/file-graphs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ repositoryUrl, ...(ref ? { ref } : {}) }),
        signal,
      })
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") throw error
      throw new Error("The file graph service could not be reached.", { cause: error })
    }

    const body: unknown = await response.json().catch(() => null)
    if (!response.ok) {
      const message = typeof (body as ErrorResponse | null)?.error === "string"
        ? (body as { error: string }).error
        : "The GitHub repository could not be analyzed."
      throw new Error(message)
    }
    return parseFileGraph(body)
  }
}
