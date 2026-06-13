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

let socket: WebSocket | null = null;
const handlers = new Map<string, TerminalHandlers>();

export const api = {
  listProjects: () => json<Project[]>("/api/projects"),
  forgetProject: (id: string) => empty(`/api/projects/${encodeURIComponent(id)}`, { method: "DELETE" }),
  listTerminals: (projectId = "") =>
    json<TerminalSession[]>(`/api/terminals${projectId ? `?projectId=${encodeURIComponent(projectId)}` : ""}`),
  startTerminal: (projectId: string, cols: number, rows: number) =>
    json<TerminalSession>("/api/terminals", { method: "POST", body: { projectId, cols, rows } }),
  writeTerminal: (id: string, data: string) =>
    empty(`/api/terminals/${encodeURIComponent(id)}/input`, { method: "POST", body: { data } }),
  resizeTerminal: (id: string, cols: number, rows: number) =>
    empty(`/api/terminals/${encodeURIComponent(id)}/resize`, { method: "POST", body: { cols, rows } }),
  closeTerminal: (id: string) => empty(`/api/terminals/${encodeURIComponent(id)}/close`, { method: "POST" }),
  async openProject(): Promise<Project | null> {
    const path = window.prompt("Project path");
    return path ? json<Project>("/api/projects", { method: "POST", body: { path } }) : null;
  },
  subscribeTerminal(id: string, handler: TerminalHandlers): () => void {
    handlers.set(id, handler);
    openSocket();
    return () => {
      handlers.delete(id);
      if (handlers.size === 0) {
        socket?.close();
        socket = null;
      }
    };
  },
};

type JSONInit = Omit<RequestInit, "body"> & { body?: unknown };

async function json<T>(path: string, init?: JSONInit): Promise<T> {
  const response = await request(path, init);
  return response.json() as Promise<T>;
}

async function empty(path: string, init?: JSONInit): Promise<void> {
  await request(path, init);
}

async function request(path: string, init: JSONInit = {}): Promise<Response> {
  const response = await fetch(path, {
    ...init,
    headers: { "Content-Type": "application/json", ...init.headers },
    body: init.body === undefined ? undefined : JSON.stringify(init.body),
  });
  if (!response.ok) throw new Error((await response.text()) || `${path} failed with ${response.status}`);
  return response;
}

function openSocket() {
  if (socket && socket.readyState < WebSocket.CLOSING) return;
  const url = new URL("/api/events", window.location.origin);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  socket = new WebSocket(url);
  socket.addEventListener("message", (message) => {
    const event = JSON.parse(message.data) as Record<string, unknown>;
    const handler = handlers.get(String(event.terminalId ?? ""));
    if (event.type === "terminal:data") handler?.onData(String(event.data ?? ""));
    if (event.type === "terminal:exit") handler?.onExit(typeof event.exitCode === "number" ? event.exitCode : null);
    if (event.type === "terminal:error") handler?.onError(String(event.error ?? "terminal error"));
  });
}
