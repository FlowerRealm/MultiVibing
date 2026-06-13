import { FitAddon } from "@xterm/addon-fit";
import { Terminal as XTerm } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { useEffect, useRef } from "react";
import { api, type TerminalSession } from "../bridge";

interface TerminalPaneProps {
  session: TerminalSession;
  visible: boolean;
  onExit(terminalId: string, exitCode: number | null): void;
  onError(error: string): void;
}

export function TerminalPane({ session, visible, onExit, onError }: TerminalPaneProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<XTerm | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const visibleRef = useRef(visible);

  useEffect(() => {
    visibleRef.current = visible;
  }, [visible]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const terminal = new XTerm({
      cursorBlink: true,
      convertEol: true,
      fontFamily: '"JetBrains Mono", "SFMono-Regular", Consolas, monospace',
      fontSize: 13,
      theme: { background: "#101416", foreground: "#e6eee9", cursor: "#f4d06f", selectionBackground: "#315b66" },
    });
    const fit = new FitAddon();
    terminal.loadAddon(fit);
    terminal.open(container);
    terminalRef.current = terminal;
    fitRef.current = fit;

    const fitTerminal = () => {
      if (!visibleRef.current || container.offsetParent === null) return;
      try {
        fit.fit();
        void api.resizeTerminal(session.id, terminal.cols, terminal.rows);
      } catch {}
    };

    let resizeTimer: ReturnType<typeof setTimeout> | null = null;
    const dataDisposable = terminal.onData((data) => void api.writeTerminal(session.id, data));
    const observer = new ResizeObserver(() => {
      if (resizeTimer !== null) clearTimeout(resizeTimer);
      resizeTimer = setTimeout(fitTerminal, 150);
    });
    observer.observe(container);
    window.requestAnimationFrame(fitTerminal);

    let unsubscribe: (() => void) | undefined;
    let disposed = false;
    let snapshotLoaded = false;
    let lastSeq = 0;
    let pendingData: Array<{ data: string; seq: number }> = [];
    let pendingExit: number | null | undefined;
    const loadSnapshot = () => {
      api
        .getTerminalSnapshot(session.id)
        .then((snapshot) => {
          if (disposed) return;
          if (snapshot.history) terminal.write(snapshot.history);
          lastSeq = snapshot.lastSeq;
          snapshotLoaded = true;
          for (const event of pendingData) {
            writeData(event.data, event.seq);
          }
          pendingData = [];
          if (pendingExit !== undefined) {
            writeExit(terminal, pendingExit);
          } else if (snapshot.session.status === "exited") {
            writeExit(terminal, snapshot.session.exitCode ?? null);
            onExit(session.id, snapshot.session.exitCode ?? null);
          }
        })
        .catch((err) => {
          if (disposed) return;
          snapshotLoaded = true;
          pendingData = [];
          onError(err instanceof Error ? err.message : String(err));
        });
    };
    const writeData = (data: string, seq: number) => {
      if (!snapshotLoaded) {
        pendingData.push({ data, seq });
        return;
      }
      if (seq > lastSeq) {
        terminal.write(data);
        lastSeq = seq;
      }
    };

    if (session.status === "running") {
      unsubscribe = api.subscribeTerminal(session.id, {
        onData: writeData,
        onExit: (exitCode) => {
          if (!snapshotLoaded) {
            pendingExit = exitCode;
          } else {
            writeExit(terminal, exitCode);
          }
          onExit(session.id, exitCode);
        },
        onError,
        onReady: loadSnapshot,
      });
    } else {
      loadSnapshot();
    }

    return () => {
      disposed = true;
      if (resizeTimer !== null) clearTimeout(resizeTimer);
      dataDisposable.dispose();
      observer.disconnect();
      unsubscribe?.();
      terminal.dispose();
      terminalRef.current = null;
      fitRef.current = null;
    };
  }, [session.id, session.status, onExit, onError]);

  useEffect(() => {
    if (!visible) return;
    window.requestAnimationFrame(() => {
      try {
        fitRef.current?.fit();
        const t = terminalRef.current;
        if (t) void api.resizeTerminal(session.id, t.cols, t.rows);
      } catch {}
    });
  }, [visible, session.id]);

  return <div className={visible ? "terminal-pane active" : "terminal-pane"} ref={containerRef} />;
}

function writeExit(terminal: XTerm, exitCode: number | null) {
  terminal.writeln("");
  terminal.writeln(`[进程已退出${exitCode === null ? "" : `: ${exitCode}`}]`);
}
