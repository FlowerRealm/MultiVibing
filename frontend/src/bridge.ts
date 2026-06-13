export interface Project {
  id: string;
  name: string;
  path: string;
  lastOpenedAt: string;
}

export interface TerminalSession {
  id: string;
  projectId: string;
  cwd: string;
  pid: number;
  status: "running" | "exited";
  exitCode?: number | null;
}

export interface TerminalHandlers {
  onData(data: string): void;
  onExit(exitCode: number | null): void;
  onError(message: string): void;
}

export interface Client {
  listProjects(): Promise<Project[]>;
  openProjectDialog(): Promise<Project | null>;
  forgetProject(id: string): Promise<void>;
  listTerminals(projectId?: string): Promise<TerminalSession[]>;
  startTerminal(projectId: string, cols: number, rows: number): Promise<TerminalSession>;
  writeTerminal(id: string, data: string): Promise<void>;
  resizeTerminal(id: string, cols: number, rows: number): Promise<void>;
  closeTerminal(id: string): Promise<void>;
  subscribeTerminal(id: string, handlers: TerminalHandlers): () => void;
}

export function createClient(): Client {
  return new BrowserClient(import.meta.env.VITE_API_BASE ?? "");
}

type JSONRequestInit = Omit<RequestInit, "body"> & { body?: unknown };

class BrowserClient implements Client {
  private socket: WebSocket | null = null;
  private handlers = new Map<string, TerminalHandlers>();

  constructor(private readonly apiBase: string) {}

  async listProjects(): Promise<Project[]> {
    return this.json<Project[]>("/api/projects");
  }

  async openProjectDialog(): Promise<Project | null> {
    const path = window.prompt("Project path");
    if (!path) return null;
    return this.json<Project>("/api/projects", { method: "POST", body: { path } });
  }

  forgetProject(id: string): Promise<void> {
    return this.empty(`/api/projects/${encodeURIComponent(id)}`, { method: "DELETE" });
  }

  async listTerminals(projectId = ""): Promise<TerminalSession[]> {
    const query = projectId ? `?projectId=${encodeURIComponent(projectId)}` : "";
    return this.json<TerminalSession[]>(`/api/terminals${query}`);
  }

  startTerminal(projectId: string, cols: number, rows: number): Promise<TerminalSession> {
    return this.json<TerminalSession>("/api/terminals", { method: "POST", body: { projectId, cols, rows } });
  }

  writeTerminal(id: string, data: string): Promise<void> {
    return this.empty(`/api/terminals/${encodeURIComponent(id)}/input`, { method: "POST", body: { data } });
  }

  resizeTerminal(id: string, cols: number, rows: number): Promise<void> {
    return this.empty(`/api/terminals/${encodeURIComponent(id)}/resize`, { method: "POST", body: { cols, rows } });
  }

  closeTerminal(id: string): Promise<void> {
    return this.empty(`/api/terminals/${encodeURIComponent(id)}/close`, { method: "POST" });
  }

  subscribeTerminal(id: string, handlers: TerminalHandlers): () => void {
    this.handlers.set(id, handlers);
    this.ensureSocket();
    return () => {
      this.handlers.delete(id);
      if (this.handlers.size === 0) {
        this.socket?.close();
        this.socket = null;
      }
    };
  }

  private ensureSocket() {
    if (this.socket && this.socket.readyState < WebSocket.CLOSING) return;
    const socket = new WebSocket(this.wsURL("/api/events"));
    socket.addEventListener("message", (message) => {
      const event = JSON.parse(message.data) as Record<string, unknown>;
      const id = String(event.terminalId ?? "");
      const handlers = this.handlers.get(id);
      if (!handlers) return;
      switch (event.type) {
        case "terminal:data":
          handlers.onData(String(event.data ?? ""));
          break;
        case "terminal:exit":
          handlers.onExit(typeof event.exitCode === "number" ? event.exitCode : null);
          break;
        case "terminal:error":
          handlers.onError(String(event.error ?? "terminal error"));
          break;
      }
    });
    this.socket = socket;
  }

  private async json<T>(path: string, init?: JSONRequestInit): Promise<T> {
    const response = await this.request(path, init);
    return response.json() as Promise<T>;
  }

  private async empty(path: string, init?: JSONRequestInit): Promise<void> {
    await this.request(path, init);
  }

  private async request(path: string, init: JSONRequestInit = {}): Promise<Response> {
    const response = await fetch(`${this.apiBase}${path}`, {
      ...init,
      headers: { "Content-Type": "application/json", ...init.headers },
      body: init.body === undefined ? undefined : JSON.stringify(init.body),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || `${path} failed with ${response.status}`);
    }
    return response;
  }

  private wsURL(path: string): string {
    const base = this.apiBase || window.location.origin;
    const url = new URL(path, base);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    return url.toString();
  }
}
