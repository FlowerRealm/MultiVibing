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
    if (session.status === "running") {
      unsubscribe = api.subscribeTerminal(session.id, {
        onData: (data) => terminal.write(data),
        onExit: (exitCode) => {
          terminal.writeln("");
          terminal.writeln(`[进程已退出${exitCode === null ? "" : `: ${exitCode}`}]`);
          onExit(session.id, exitCode);
        },
        onError,
      });
    }

    return () => {
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
