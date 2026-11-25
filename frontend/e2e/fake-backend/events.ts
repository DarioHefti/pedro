type Listener = { callback: (...data: unknown[]) => void; remaining: number }

/** The Wails runtime event bus (EventsOnMultiple / EventsOff / EventsEmit), in memory. */
export class FakeEventBus {
  private listeners = new Map<string, Listener[]>()

  /** Registers a listener; `maxCallbacks` of -1 means unlimited. Returns its cancel function. */
  on(name: string, callback: (...data: unknown[]) => void, maxCallbacks = -1): () => void {
    const listener: Listener = { callback, remaining: maxCallbacks }
    this.listeners.set(name, [...(this.listeners.get(name) ?? []), listener])
    return () => {
      this.listeners.set(
        name,
        (this.listeners.get(name) ?? []).filter(l => l !== listener),
      )
    }
  }

  off(...names: string[]): void {
    for (const name of names) this.listeners.delete(name)
  }

  offAll(): void {
    this.listeners.clear()
  }

  emit(name: string, ...data: unknown[]): void {
    const current = this.listeners.get(name) ?? []
    for (const l of current) {
      if (l.remaining === 0) continue
      if (l.remaining > 0) l.remaining -= 1
      l.callback(...data)
    }
    this.listeners.set(
      name,
      (this.listeners.get(name) ?? []).filter(l => l.remaining !== 0),
    )
  }

  count(name: string): number {
    return this.listeners.get(name)?.length ?? 0
  }
}
