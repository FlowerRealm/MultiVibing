import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import type { Client, Project, TerminalHandlers, TerminalSession } from "./bridge";

vi.mock("@xterm/xterm", () => ({
  Terminal: class {
    cols = 100;
    rows = 30;
    loadAddon() {}
    open() {}
    onData() {
      return { dispose() {} };
    }
    write() {}
    writeln() {}
    dispose() {}
  },
}));

vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class {
    fit() {}
  },
}));

describe("App", () => {
  afterEach(() => {
    cleanup();
  });

  it("opens projects, starts terminals, and closes terminal tabs", async () => {
    const client = new FakeClient();
    const user = userEvent.setup();

    render(<App client={client} />);

    expect(await screen.findByRole("heading", { name: "alpha" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "浏览项目" }));
    expect(await screen.findByRole("heading", { name: "opened-project" })).toBeInTheDocument();

    await user.click(screen.getAllByRole("button", { name: "新终端" })[0]);
    const tab = await screen.findByRole("tab", { name: /终端 1/ });
    expect(within(tab).getByText("PID 1001")).toBeInTheDocument();
    expect(client.startedProjectId).toBe("opened-project");

    await user.click(screen.getByRole("button", { name: "关闭终端 1" }));
    expect(screen.queryByRole("tab", { name: /终端 1/ })).not.toBeInTheDocument();
    expect(client.closedTerminalId).toBe("term-1");
  });
});

class FakeClient implements Client {
  startedProjectId = "";
  closedTerminalId = "";

  private projects: Project[] = [
    {
      id: "alpha",
      name: "alpha",
      path: "/tmp/alpha",
      lastOpenedAt: "2026-06-07T10:00:00Z",
    },
  ];

  private terminals: TerminalSession[] = [];

  async listProjects(): Promise<Project[]> {
    return this.projects;
  }

  async openProjectDialog(): Promise<Project | null> {
    const project = {
      id: "opened-project",
      name: "opened-project",
      path: "/tmp/opened-project",
      lastOpenedAt: "2026-06-07T10:01:00Z",
    };
    this.projects = [project, ...this.projects];
    return project;
  }

  async forgetProject(): Promise<void> {}

  async listTerminals(projectId?: string): Promise<TerminalSession[]> {
    return projectId ? this.terminals.filter((terminal) => terminal.projectId === projectId) : this.terminals;
  }

  async startTerminal(projectId: string): Promise<TerminalSession> {
    this.startedProjectId = projectId;
    const session: TerminalSession = {
      id: "term-1",
      projectId,
      cwd: "/tmp/opened-project",
      pid: 1001,
      status: "running",
    };
    this.terminals = [...this.terminals, session];
    return session;
  }

  async writeTerminal(): Promise<void> {}

  async resizeTerminal(): Promise<void> {}

  async closeTerminal(id: string): Promise<void> {
    this.closedTerminalId = id;
    this.terminals = this.terminals.filter((terminal) => terminal.id !== id);
  }

  subscribeTerminal(_id: string, _handlers: TerminalHandlers): () => void {
    return () => {};
  }
}
