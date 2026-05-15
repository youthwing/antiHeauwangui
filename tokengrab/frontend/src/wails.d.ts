// Wails-injected globals. The IDE doesn't know about them at TS-build time.

export {}

declare global {
  interface Window {
    go: {
      main: {
        App: {
          Start(): Promise<void>
          Cancel(): Promise<void>
          Cleanup(): Promise<void>
          Reset(): Promise<void>
          GetPhase(): Promise<'idle' | 'capturing' | 'captured' | 'error'>
          OpenWanguiActivate(url: string, token: string, invite: string): Promise<void>
          CheckResidual(): Promise<boolean>
          CleanResidual(): Promise<void>
          SetClipboard(s: string): Promise<void>
          CAInstalled(): Promise<boolean>
          UninstallPersistentCA(): Promise<void>
        }
      }
    }
    runtime: {
      EventsOn(evt: string, cb: (...data: any[]) => void): () => void
      EventsOff(evt: string): void
    }
  }
}
