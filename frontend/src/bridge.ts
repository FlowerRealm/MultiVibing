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

declare global {
  interface Window {
    go?: { desktop?: { Bridge?: Record<string, (...args: unknown[]) => Promise<unknown>> } };
    runtime?: { EventsOn(name: string, cb: (payload: Record<string, unknown>) => void): (() => void) | void };
  }
}

export function createClient(): Client {
  const b = window.go?.desktop?.Bridge;
  if (!b) {
    const fail = async (): Promise<never> => { throw new Error("Wails bridge unavailable — run wails dev"); };
    return { listProjects: fail, openProjectDialog: fail, forgetProject: fail, listTerminals: fail,
             startTerminal: fail, writeTerminal: fail, resizeTerminal: fail, closeTerminal: fail,
             subscribeTerminal: () => () => {} };
  }
  return new WailsClient(b, window.runtime);
}

type Bridge = NonNullable<NonNullable<Window["go"]>["desktop"]>["Bridge"];
type Runtime = NonNullable<Window["runtime"]>;

class WailsClient implements Client {
  constructor(private b: Bridge, private rt?: Runtime) {}

  async listProjects() { return ((await this.b!.ListProjects()) as Project[] | null) ?? []; }
  openProjectDialog() { return this.b!.OpenProjectDialog() as Promise<Project | null>; }
  forgetProject(id: string) { return this.b!.ForgetProject(id) as Promise<void>; }
  async listTerminals(projectId = "") { return ((await this.b!.ListTerminals(projectId)) as TerminalSession[] | null) ?? []; }
  startTerminal(projectId: string, cols: number, rows: number) { return this.b!.StartTerminal(projectId, cols, rows) as Promise<TerminalSession>; }
  writeTerminal(id: string, data: string) { return this.b!.WriteTerminal(id, data) as Promise<void>; }
  resizeTerminal(id: string, cols: number, rows: number) { return this.b!.ResizeTerminal(id, cols, rows) as Promise<void>; }
  closeTerminal(id: string) { return this.b!.CloseTerminal(id) as Promise<void>; }

  subscribeTerminal(id: string, handlers: TerminalHandlers): () => void {
    const on = (name: string, cb: (p: Record<string, unknown>) => void) => {
      const off = this.rt?.EventsOn(name, (p) => { if (p?.terminalId === id) cb(p); });
      return typeof off === "function" ? off : () => {};
    };
    const offs = [
      on("terminal:data", (p) => handlers.onData(String(p.data ?? ""))),
      on("terminal:exit", (p) => handlers.onExit(typeof p.exitCode === "number" ? p.exitCode : null)),
      on("terminal:error", (p) => handlers.onError(String(p.error ?? "terminal error"))),
    ];
    return () => offs.forEach((f) => f());
  }
}
