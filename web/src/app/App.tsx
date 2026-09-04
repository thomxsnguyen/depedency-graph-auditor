import { Component, type ErrorInfo, type ReactNode } from "react"
import { OperationsDashboard } from "./OperationsDashboard"

interface ErrorBoundaryState {
  error: Error | null
}

class ErrorBoundary extends Component<{ children: ReactNode }, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Operations dashboard failed", error, info)
  }

  render() {
    if (this.state.error) {
      return (
        <main className="centered-state" role="alert">
          <div className="state-panel">
            <p className="eyebrow">Distributed job queue</p>
            <h1>The operations dashboard could not be displayed.</h1>
            <p>Refresh the page to reconnect to the job API.</p>
            <button className="button button--primary" onClick={() => window.location.reload()}>
              Reload dashboard
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
      <OperationsDashboard />
    </ErrorBoundary>
  )
}
