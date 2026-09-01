import { Component, type ErrorInfo, type ReactNode } from "react"
import { AuditWorkspace } from "./AuditWorkspace"

interface ErrorBoundaryState {
  error: Error | null
}

class ErrorBoundary extends Component<{ children: ReactNode }, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Dependency Audit Studio failed", error, info)
  }

  render() {
    if (this.state.error) {
      return (
        <main className="centered-state" role="alert">
          <div className="state-panel">
            <p className="eyebrow">Dependency Audit Studio</p>
            <h1>The audit view could not be displayed.</h1>
            <p>Refresh the page to reload the local demo.</p>
            <button className="button button--primary" onClick={() => window.location.reload()}>
              Reload demo
            </button>
          </div>
        </main>
      )
    }

    return this.props.children
  }
}

export function App() {
  return (
    <ErrorBoundary>
      <AuditWorkspace />
    </ErrorBoundary>
  )
}
