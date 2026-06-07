import { useEffect, useMemo, useState } from "react";
import { createTransport, type AppEvent, type CodexStatus, type Health } from "./transport";

export function App() {
  const transport = useMemo(() => createTransport(), []);
  const [health, setHealth] = useState<Health | null>(null);
  const [codex, setCodex] = useState<CodexStatus | null>(null);
  const [events, setEvents] = useState<AppEvent[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;

    async function load() {
      try {
        const [nextHealth, nextCodex] = await Promise.all([
          transport.health(),
          transport.codexStatus(),
        ]);
        if (!active) return;
        setHealth(nextHealth);
        setCodex(nextCodex);
      } catch (err) {
        if (!active) return;
        setError(err instanceof Error ? err.message : String(err));
      }
    }

    const unsubscribe = transport.subscribe((event) => {
      setEvents((current) => [event, ...current].slice(0, 24));
    });

    load();
    const timer = window.setInterval(load, 5000);
    return () => {
      active = false;
      window.clearInterval(timer);
      unsubscribe();
    };
  }, [transport]);

  return (
    <main className="shell">
      <section className="panel header-panel">
        <div>
          <p className="eyebrow">Codex app-server client</p>
          <h1>MultiVibing</h1>
        </div>
        <span className="mode">{health?.mode ?? "loading"}</span>
      </section>

      {error ? <section className="panel error">{error}</section> : null}

      <section className="grid">
        <article className="panel">
          <h2>Application</h2>
          <dl>
            <dt>Status</dt>
            <dd>{health?.ok ? "Ready" : "Loading"}</dd>
            <dt>Version</dt>
            <dd>{health?.version ?? "-"}</dd>
            <dt>Started</dt>
            <dd>{health?.startedAt ?? "-"}</dd>
          </dl>
        </article>

        <article className="panel">
          <h2>Codex</h2>
          <dl>
            <dt>Installed</dt>
            <dd>{codex?.available ? "Yes" : "No"}</dd>
            <dt>Version</dt>
            <dd>{codex?.version ?? "-"}</dd>
            <dt>app-server</dt>
            <dd>{codex?.running ? `Running pid ${codex.pid}` : "Stopped"}</dd>
          </dl>
        </article>
      </section>

      <section className="panel">
        <h2>Events</h2>
        {events.length === 0 ? (
          <p className="muted">No events yet.</p>
        ) : (
          <ol className="events">
            {events.map((event, index) => (
              <li key={`${event.timestamp}-${index}`}>
                <strong>{event.type}</strong>
                <span>{event.message}</span>
              </li>
            ))}
          </ol>
        )}
      </section>
    </main>
  );
}
