export interface Health {
  ok: boolean;
  name: string;
  version: string;
  mode: string;
  startedAt: string;
}

export interface CodexStatus {
  available: boolean;
  running: boolean;
  version?: string;
  pid?: number;
  error?: string;
}

export interface AppEvent {
  type: string;
  message: string;
  data?: Record<string, unknown>;
  timestamp: string;
}

export interface Transport {
  health(): Promise<Health>;
  codexStatus(): Promise<CodexStatus>;
  subscribe(onEvent: (event: AppEvent) => void): () => void;
}

export function createTransport(): Transport {
  return new BrowserTransport(import.meta.env.VITE_API_BASE ?? "");
}

class BrowserTransport implements Transport {
  constructor(private readonly apiBase: string) {}

  async health(): Promise<Health> {
    return this.getJSON("/api/health");
  }

  async codexStatus(): Promise<CodexStatus> {
    return this.getJSON("/api/codex/status");
  }

  subscribe(onEvent: (event: AppEvent) => void): () => void {
    const url = this.wsURL("/api/events");
    const socket = new WebSocket(url);
    socket.addEventListener("message", (message) => {
      onEvent(JSON.parse(message.data) as AppEvent);
    });
    return () => socket.close();
  }

  private async getJSON<T>(path: string): Promise<T> {
    const response = await fetch(`${this.apiBase}${path}`);
    if (!response.ok) {
      throw new Error(`${path} failed with ${response.status}`);
    }
    return response.json() as Promise<T>;
  }

  private wsURL(path: string): string {
    const base = this.apiBase || window.location.origin;
    const url = new URL(path, base);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    return url.toString();
  }
}
