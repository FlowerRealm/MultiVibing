import { FolderOpen, Plus, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { createClient, type Client, type Project, type TerminalSession } from "./bridge";
import { TerminalPane } from "./terminal/TerminalPane";

interface AppProps {
  client?: Client;
}

export function App({ client: providedClient }: AppProps = {}) {
  const client = useMemo(() => providedClient ?? createClient(), [providedClient]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [terminals, setTerminals] = useState<TerminalSession[]>([]);
  const [activeProjectId, setActiveProjectId] = useState<string | null>(null);
  const [activeTerminalId, setActiveTerminalId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    const [nextProjects, nextTerminals] = await Promise.all([client.listProjects(), client.listTerminals()]);
    setProjects(nextProjects);
    setTerminals(nextTerminals);
    setActiveProjectId((current) => (current && nextProjects.some((project) => project.id === current) ? current : nextProjects[0]?.id ?? null));
  }, [client]);

  useEffect(() => {
    load().catch((err) => setError(errorText(err)));
  }, [load]);

  const activeProject = projects.find((project) => project.id === activeProjectId) ?? null;
  const projectTerminals = activeProject ? terminals.filter((terminal) => terminal.projectId === activeProject.id) : [];
  const activeTerminal =
    projectTerminals.find((terminal) => terminal.id === activeTerminalId) ?? projectTerminals[0] ?? null;

  async function openProject() {
    try {
      setError(null);
      const project = await client.openProjectDialog();
      if (!project) return;
      setProjects((current) => upsertProject(current, project));
      setActiveProjectId(project.id);
      const nextTerminals = await client.listTerminals(project.id);
      setTerminals((current) => mergeProjectTerminals(current, project.id, nextTerminals));
      setActiveTerminalId(nextTerminals[0]?.id ?? null);
    } catch (err) {
      setError(errorText(err));
    }
  }

  async function forgetProject(projectId: string) {
    try {
      setError(null);
      await client.forgetProject(projectId);
      setProjects((current) => {
        const next = current.filter((project) => project.id !== projectId);
        if (activeProjectId === projectId) {
          setActiveProjectId(next[0]?.id ?? null);
          setActiveTerminalId(null);
        }
        return next;
      });
      setTerminals((current) => current.filter((terminal) => terminal.projectId !== projectId));
    } catch (err) {
      setError(errorText(err));
    }
  }

  async function startTerminal() {
    if (!activeProject) return;
    try {
      setError(null);
      const session = await client.startTerminal(activeProject.id, 100, 30);
      setTerminals((current) => upsertTerminal(current, session));
      setActiveTerminalId(session.id);
    } catch (err) {
      setError(errorText(err));
    }
  }

  async function closeTerminal(id: string) {
    try {
      setError(null);
      await client.closeTerminal(id);
      setTerminals((current) => {
        const closed = current.find((terminal) => terminal.id === id);
        const next = current.filter((terminal) => terminal.id !== id);
        if (activeTerminalId === id) {
          setActiveTerminalId(next.find((terminal) => terminal.projectId === closed?.projectId)?.id ?? null);
        }
        return next;
      });
    } catch (err) {
      setError(errorText(err));
    }
  }

  function markExited(id: string, exitCode: number | null) {
    setTerminals((current) =>
      current.map((terminal) =>
        terminal.id === id
          ? {
              ...terminal,
              status: "exited",
              exitCode,
            }
          : terminal,
      ),
    );
  }

  return (
    <main className="workspace">
      <aside className="sidebar">
        <div className="brand">
          <span className="product-name">MultiVibing</span>
        </div>

        <div className="project-section-label">项目</div>

        <nav className="project-list" aria-label="项目">
          {projects.length === 0 ? <p className="empty">暂无项目。</p> : null}
          {projects.map((project) => (
            <div className="project-row" key={project.id}>
              <button
                className={project.id === activeProjectId ? "project-button active" : "project-button"}
                type="button"
                onClick={() => {
                  setActiveProjectId(project.id);
                  setActiveTerminalId(null);
                }}
              >
                <span>{project.name}</span>
                <small>{project.path}</small>
              </button>
              <button className="icon-button" type="button" aria-label={`移除 ${project.name}`} onClick={() => void forgetProject(project.id)}>
                <X size={15} aria-hidden="true" />
              </button>
            </div>
          ))}
        </nav>

        <div className="sidebar-footer">
          <button className="tool-button" type="button" aria-label="浏览项目" title="浏览项目" onClick={() => void openProject()}>
            <FolderOpen size={17} aria-hidden="true" />
          </button>
        </div>
      </aside>

      <section className="main-panel">
        <header className="project-header">
          <div>
            <p className="eyebrow">{activeProject?.path ?? "未选择项目"}</p>
            <h2>{activeProject?.name ?? "打开一个项目"}</h2>
          </div>
          <button className="primary-button compact" type="button" onClick={() => void startTerminal()} disabled={!activeProject}>
            <Plus size={16} aria-hidden="true" />
            新终端
          </button>
        </header>

        {error ? <div className="error">{error}</div> : null}

        {activeProject ? (
          <>
            <div className="tab-strip" role="tablist" aria-label="终端">
              {projectTerminals.length === 0 ? <span className="empty inline">暂无终端。</span> : null}
              {projectTerminals.map((terminal, index) => (
                <div className={terminal.id === activeTerminal?.id ? "terminal-tab active" : "terminal-tab"} key={terminal.id}>
                  <button
                    type="button"
                    role="tab"
                    aria-selected={terminal.id === activeTerminal?.id}
                    onClick={() => setActiveTerminalId(terminal.id)}
                  >
                    <span>终端 {index + 1}</span>
                    <small>{terminal.status === "running" ? `PID ${terminal.pid}` : "已退出"}</small>
                  </button>
                  <button className="tab-close" type="button" aria-label={`关闭终端 ${index + 1}`} onClick={() => void closeTerminal(terminal.id)}>
                    <X size={14} aria-hidden="true" />
                  </button>
                </div>
              ))}
            </div>

            <div className="terminal-stage">
              {projectTerminals.map((terminal) => (
                <TerminalPane
                  key={terminal.id}
                  session={terminal}
                  visible={terminal.id === activeTerminal?.id}
                  client={client}
                  onExit={markExited}
                  onError={setError}
                />
              ))}
              {projectTerminals.length === 0 ? (
                <div className="terminal-empty">
                  <button className="primary-button" type="button" onClick={() => void startTerminal()}>
                    <Plus size={16} aria-hidden="true" />
                    新终端
                  </button>
                </div>
              ) : null}
            </div>
          </>
        ) : (
          <div className="empty-state">
            <p className="empty">从左下角添加项目。</p>
          </div>
        )}
      </section>
    </main>
  );
}

function upsertProject(projects: Project[], project: Project): Project[] {
  return [project, ...projects.filter((current) => current.id !== project.id)];
}

function upsertTerminal(terminals: TerminalSession[], terminal: TerminalSession): TerminalSession[] {
  const index = terminals.findIndex((current) => current.id === terminal.id);
  if (index === -1) {
    return [...terminals, terminal];
  }
  const next = terminals.slice();
  next[index] = terminal;
  return next;
}

function mergeProjectTerminals(
  terminals: TerminalSession[],
  projectId: string,
  projectTerminals: TerminalSession[],
): TerminalSession[] {
  return [...terminals.filter((terminal) => terminal.projectId !== projectId), ...projectTerminals];
}

function errorText(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
