import type { ReactNode } from "react";
import type { Activity } from "../api/generated/models";

export function Section({ title, hint, children }: { title: string; hint?: string | undefined; children: ReactNode }) {
  return (
    <section className="section-card">
      <div className="section-title">
        <div>
          <h2>{title}</h2>
          {hint && <p>{hint}</p>}
        </div>
      </div>
      {children}
    </section>
  );
}

export function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="stat">
      <small>{label}</small>
      <strong>{value}</strong>
    </div>
  );
}

export function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="info">
      <small>{label}</small>
      <strong>{value}</strong>
    </div>
  );
}

export function Notice({ children, tone = "info" }: { children: ReactNode; tone?: "info" | "error" | "success" }) {
  return (
    <p className={`notice ${tone}`} role={tone === "error" ? "alert" : "status"}>
      {children}
    </p>
  );
}

export function Code({ value }: { value: unknown }) {
  const text = typeof value === "string" ? value : JSON.stringify(value, null, 2);
  return <pre className="code-block">{text}</pre>;
}

export function QueryState({
  pending,
  error,
  empty,
}: {
  pending: boolean;
  error: Error | null;
  empty?: boolean | undefined;
}) {
  if (pending)
    return (
      <p className="empty" role="status">
        Loading…
      </p>
    );
  if (error) return <Notice tone="error">{error.message}</Notice>;
  if (empty) return <p className="empty">Nothing to show.</p>;
  return null;
}

export function ActivityTable({
  items,
  onSelect,
  selected,
}: {
  items: Activity[];
  onSelect?: ((id: string) => void) | undefined;
  selected?: string | undefined;
}) {
  if (items.length === 0) return <p className="empty">No retained delivery activity yet.</p>;
  return (
    <div className="activity-table">
      {items.map((item) => (
        <button
          className={selected === item.attemptId ? "activity-row selected" : "activity-row"}
          key={item.attemptId}
          onClick={() => onSelect?.(item.attemptId)}
          disabled={!onSelect}
          type="button"
        >
          <span>
            <strong>
              {item.eventType}@{item.eventVersion}
            </strong>
            <small>{new Date(item.createdAt).toLocaleString()}</small>
          </span>
          <span className={`outcome ${item.outcome}`}>{item.outcome}</span>
          <code>{item.status ?? "—"}</code>
          <code>{item.durationMs.toFixed(1)}ms</code>
        </button>
      ))}
    </div>
  );
}

export function friendly(value: string): string {
  return value.replaceAll("_", " ").replace(/\b\w/g, (letter) => letter.toUpperCase());
}
