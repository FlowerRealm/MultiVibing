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

interface WailsBridge {
  ListProjects(): Promise<Project[] | null>;
  OpenProjectDialog(): Promise<Project | null>;
  ForgetProject(id: string): Promise<void>;
  ListTerminals(projectId: string): Promise<TerminalSession[] | null>;
  StartTerminal(projectId: string, cols: number, rows: number): Promise<TerminalSession>;
  WriteTerminal(id: string, data: string): Promise<void>;
  ResizeTerminal(id: string, cols: number, rows: number): Promise<void>;
  CloseTerminal(id: string): Promise<void>;
}

interface WailsRuntime {
  EventsOn(eventName: string, callback: (payload: TerminalPayload) => void): (() => void) | void;
}

interface TerminalPayload {
  terminalId?: string;
  data?: string;
  exitCode?: number | null;
  error?: string;
}

declare global {
  interface Window {
    go?: {
      desktop?: {
        Bridge?: WailsBridge;
      };
    };
    runtime?: WailsRuntime;
  }
}

export function createClient(): Client {
  const bridge = window.go?.desktop?.Bridge;
  if (!bridge) {
    return missingClient();
  }
  return new WailsClient(bridge, window.runtime);
}

class WailsClient implements Client {
  constructor(
    private readonly bridge: WailsBridge,
    private readonly runtime?: WailsRuntime,
  ) {}

  async listProjects(): Promise<Project[]> {
    return arrayOrEmpty(await this.bridge.ListProjects());
  }

  openProjectDialog(): Promise<Project | null> {
    return this.bridge.OpenProjectDialog();
  }

  forgetProject(id: string): Promise<void> {
    return this.bridge.ForgetProject(id);
  }

  async listTerminals(projectId = ""): Promise<TerminalSession[]> {
    return arrayOrEmpty(await this.bridge.ListTerminals(projectId));
  }

  startTerminal(projectId: string, cols: number, rows: number): Promise<TerminalSession> {
    return this.bridge.StartTerminal(projectId, cols, rows);
  }

  writeTerminal(id: string, data: string): Promise<void> {
    return this.bridge.WriteTerminal(id, data);
  }

  resizeTerminal(id: string, cols: number, rows: number): Promise<void> {
    return this.bridge.ResizeTerminal(id, cols, rows);
  }

  closeTerminal(id: string): Promise<void> {
    return this.bridge.CloseTerminal(id);
  }

  subscribeTerminal(id: string, handlers: TerminalHandlers): () => void {
    const offData = this.on("terminal:data", id, (payload) => handlers.onData(String(payload.data ?? "")));
    const offExit = this.on("terminal:exit", id, (payload) => handlers.onExit(numberOrNull(payload.exitCode)));
    const offError = this.on("terminal:error", id, (payload) => handlers.onError(String(payload.error ?? "terminal error")));
    return () => {
      offData();
      offExit();
      offError();
    };
  }

  private on(eventName: string, id: string, callback: (payload: TerminalPayload) => void): () => void {
    const cancel = this.runtime?.EventsOn(eventName, (payload) => {
      if (payload?.terminalId === id) {
        callback(payload);
      }
    });
    if (typeof cancel === "function") {
      return cancel;
    }
    return () => {};
  }
}

function arrayOrEmpty<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

function numberOrNull(value: unknown): number | null {
  return typeof value === "number" ? value : null;
}

function missingClient(): Client {
  const fail = async () => {
    throw new Error("Wails bridge is not available. Run the app with wails dev.");
  };
  return {
    listProjects: fail,
    openProjectDialog: fail,
    forgetProject: fail,
    listTerminals: fail,
    startTerminal: fail,
    writeTerminal: fail,
    resizeTerminal: fail,
    closeTerminal: fail,
    subscribeTerminal: () => () => {},
  };
}
