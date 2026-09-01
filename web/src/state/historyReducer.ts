export interface HistoryState<T> {
  past: readonly T[]
  present: T
  future: readonly T[]
}

export type HistoryAction<T, A> =
  | { type: "apply"; action: A }
  | { type: "replace"; value: T }
  | { type: "undo" }
  | { type: "redo" }

export function createHistory<T>(initial: T): HistoryState<T> {
  return { past: [], present: initial, future: [] }
}

export function historyReducer<T, A>(
  reducer: (state: T, action: A) => T,
  limit = 50,
) {
  return (state: HistoryState<T>, action: HistoryAction<T, A>): HistoryState<T> => {
    switch (action.type) {
      case "replace":
        return createHistory(action.value)
      case "undo": {
        if (state.past.length === 0) return state
        const previous = state.past[state.past.length - 1]
        return {
          past: state.past.slice(0, -1),
          present: previous,
          future: [state.present, ...state.future],
        }
      }
      case "redo": {
        if (state.future.length === 0) return state
        const next = state.future[0]
        return {
          past: [...state.past, state.present].slice(-limit),
          present: next,
          future: state.future.slice(1),
        }
      }
      case "apply": {
        const next = reducer(state.present, action.action)
        if (Object.is(next, state.present)) return state
        return {
          past: [...state.past, state.present].slice(-limit),
          present: next,
          future: [],
        }
      }
    }
  }
}
