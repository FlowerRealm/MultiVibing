import { FitAddon } from "@xterm/addon-fit";
import { Terminal as XTerm } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { useEffect, useRef } from "react";
import type { Client, TerminalSession } from "../bridge";

interface TerminalPaneProps {
  session: TerminalSession;
  visible: boolean;
  client: Client;
  onExit(terminalId: string, exitCode: number | null): void;
  onError(error: string): void;
}

export function TerminalPane({ session, visible, client, onExit, onError }: TerminalPaneProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<XTerm | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const unsubscribeRef = useRef<(() => void) | null>(null);

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
      try {
        fit.fit();
        void client.resizeTerminal(session.id, terminal.cols, terminal.rows);
      } catch { /* xterm may throw before layout is measurable */ }
    };

    const dataDisposable = terminal.onData((data) => void client.writeTerminal(session.id, data));
    const observer = new ResizeObserver(fitTerminal);
    observer.observe(container);
    window.requestAnimationFrame(fitTerminal);

    if (session.status === "running") {
      unsubscribeRef.current = client.subscribeTerminal(session.id, {
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
      dataDisposable.dispose();
      observer.disconnect();
      unsubscribeRef.current?.();
      unsubscribeRef.current = null;
      terminal.dispose();
      terminalRef.current = null;
      fitRef.current = null;
    };
  }, [client, session.id, session.status, onExit, onError]);

  useEffect(() => {
    if (!visible) return;
    window.requestAnimationFrame(() => {
      try {
        fitRef.current?.fit();
        const t = terminalRef.current;
        if (t) void client.resizeTerminal(session.id, t.cols, t.rows);
      } catch { /* layout may not be ready */ }
    });
  }, [visible, client, session.id]);

  return <div className={visible ? "terminal-pane active" : "terminal-pane"} ref={containerRef} />;
}
