import { useState, type FormEvent } from "react"
import type { GitHubFileGraphRequest } from "../../data/RepositoryFileGraphDataSource"

interface GitHubFileGraphFormProps {
  submitting: boolean
  error: string | null
  onSubmit: (request: GitHubFileGraphRequest) => void
  onChange: () => void
}

function validateGitHubRepositoryURL(value: string): string | null {
  if (!value.trim()) return "Enter a GitHub repository URL."
  try {
    const url = new URL(value.trim())
    const segments = url.pathname.replace(/^\/+|\/+$/g, "").split("/")
    const repository = segments[1]?.replace(/\.git$/, "")
    if (
      url.protocol !== "https:"
      || url.hostname.toLowerCase() !== "github.com"
      || url.port
      || url.username
      || url.password
      || url.search
      || url.hash
      || segments.length !== 2
      || !segments[0]
      || !repository
    ) {
      return "Use https://github.com/owner/repository."
    }
    return null
  } catch {
    return "Use https://github.com/owner/repository."
  }
}

export function GitHubFileGraphForm({ submitting, error, onSubmit, onChange }: GitHubFileGraphFormProps) {
  const [repositoryURL, setRepositoryURL] = useState("")
  const [ref, setRef] = useState("")
  const [validationError, setValidationError] = useState<string | null>(null)
  const displayedError = validationError ?? error

  const changed = (update: () => void) => {
    update()
    setValidationError(null)
    onChange()
  }

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const urlError = validateGitHubRepositoryURL(repositoryURL)
    if (urlError) {
      setValidationError(urlError)
      return
    }
    if (ref.length > 0 && ref.trim() === "") {
      setValidationError("Enter a branch, tag, or commit, or leave Ref blank.")
      return
    }
    onSubmit({
      repositoryUrl: repositoryURL.trim(),
      ...(ref.trim() ? { ref: ref.trim() } : {}),
    })
  }

  return (
    <form className="repository-form" onSubmit={submit} noValidate>
      <p className="eyebrow">Analyze repository</p>
      <label>
        <span>GitHub repository URL</span>
        <input
          type="url"
          value={repositoryURL}
          placeholder="https://github.com/owner/repository"
          disabled={submitting}
          aria-invalid={Boolean(displayedError)}
          onChange={(event) => changed(() => setRepositoryURL(event.target.value))}
        />
      </label>
      <label>
        <span>Ref <small>Optional</small></span>
        <input
          type="text"
          value={ref}
          placeholder="Branch, tag, or commit"
          disabled={submitting}
          onChange={(event) => changed(() => setRef(event.target.value))}
        />
      </label>
      {displayedError && <p className="repository-form__error" role="alert">{displayedError}</p>}
      <button className="button button--primary" type="submit" disabled={submitting}>
        {submitting ? "Analyzing repository…" : "Analyze repository"}
      </button>
    </form>
  )
}
