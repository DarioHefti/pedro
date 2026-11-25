import { useEffect, useState } from 'react'

interface DebouncedSearchOptions {
  delayMs: number
  /** Queries shorter than this (after trimming) are not searched. Default 1. */
  minLength?: number
}

/**
 * Runs `search` for the trimmed query once typing pauses. Results of superseded queries are
 * dropped, failures and short queries yield `empty`.
 */
export function useDebouncedSearch<T>(
  query: string,
  search: (query: string) => Promise<T>,
  empty: T,
  { delayMs, minLength = 1 }: DebouncedSearchOptions,
): T {
  const [results, setResults] = useState<T>(empty)

  useEffect(() => {
    const q = query.trim()
    if (q.length < minLength) {
      setResults(empty)
      return
    }
    let cancelled = false
    const timer = window.setTimeout(() => {
      search(q).then(
        found => {
          if (!cancelled) setResults(found)
        },
        () => {
          if (!cancelled) setResults(empty)
        },
      )
    }, delayMs)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
    // `empty` is only a fallback value: a new identity must not restart the search.
  }, [query, search, delayMs, minLength])

  return results
}
