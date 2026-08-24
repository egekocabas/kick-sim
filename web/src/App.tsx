import { useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  duplicateScenario, generateEvent, getBootstrap, getDeliveryAttempt, getScenario,
  getSimulatorKeyInfo, getSimulatorPublicKey, listRuns, listScenarios,
  replayDeliveryAttempt, rotateSimulatorKey, saveScenarioSourceCopy, triggerEvent,
  updateScenarioSource, validateEvent,
} from "./api/generated/client";
import type {
  Activity, Bootstrap, DeliveryAttemptDetail, DeliveryResult, KeyInfo,
  ScenarioDetail, ScenarioSummary,
} from "./api/generated/models";

type Page = "Dashboard" | "Events" | "Scenarios" | "Activity" | "Destinations" | "Keys & setup" | "Settings" | "API docs";
type EditorView = "form" | "json" | "http";
type JSONObject = Record<string, unknown>;

const navigation: Page[] = ["Dashboard", "Events", "Scenarios", "Activity", "Destinations", "Keys & setup", "Settings", "API docs"];

function successful<T>(response: { status: number; data: T | unknown }): T {
  if (response.status < 200 || response.status >= 300) {
    const detail = response.data as { detail?: string; title?: string };
    throw new Error(detail.detail ?? detail.title ?? `Request failed with HTTP ${response.status}`);
  }
  return response.data as T;
}

function failureMessage(data: unknown, fallback: string) {
  const detail = data as { detail?: string; errors?: Array<{ message?: string }> };
  return detail.errors?.map((item) => item.message).filter(Boolean).join("; ") || detail.detail || fallback;
}

export function App() {
  const [page, setPage] = useState<Page>("Dashboard");
  const [scenarioID, setScenarioID] = useState<string>();
  const bootstrap = useQuery({ queryKey: ["bootstrap"], queryFn: async () => successful<Bootstrap>(await getBootstrap()) });
  return <div className="studio-shell">
    <aside className="sidebar">
      <div className="brand"><span className="brand-mark">K</span><span><strong>Kick Sim</strong><small>Local Studio</small></span></div>
      <nav aria-label="Studio navigation">{navigation.map((item) => <button className={item === page ? "nav-item active" : "nav-item"} key={item} onClick={() => setPage(item)} type="button"><span className="nav-dot" />{item}</button>)}</nav>
      <div className="sidebar-foot"><span className="status-dot" /><span><strong>Local only</strong><small>127.0.0.1</small></span></div>
    </aside>
    <main>
      <header className="topbar"><div><p className="eyebrow">Kick webhook workbench</p><h1>{page}</h1></div><div className="workspace-chip"><span className="status-dot" /><span><small>Workspace</small><strong>{bootstrap.data?.workspace.path ?? "Loading…"}</strong></span></div></header>
      {bootstrap.isError && <Notice tone="error">{bootstrap.error.message}</Notice>}
      {page === "Dashboard" && <Dashboard bootstrap={bootstrap.data} onNavigate={setPage} />}
      {page === "Events" && <EventBuilder initialScenarioID={scenarioID} />}
      {page === "Scenarios" && <Scenarios onOpen={(id) => { setScenarioID(id); setPage("Events"); }} />}
      {page === "Activity" && <ActivityPage />}
      {page === "Destinations" && <Destinations bootstrap={bootstrap.data} />}
      {page === "Keys & setup" && <Keys />}
      {page === "Settings" && <Settings bootstrap={bootstrap.data} />}
      {page === "API docs" && <APIDocs />}
    </main>
  </div>;
}

function Dashboard({ bootstrap, onNavigate }: { bootstrap?: Bootstrap; onNavigate: (page: Page) => void }) {
  return <div className="page-content">
    <section className="hero-card"><p className="eyebrow">Ready on loopback</p><h2>Build, sign, send, and inspect webhooks locally.</h2><p>The browser never receives your private key. Go creates the exact bytes, headers, and signature used for delivery.</p><div className="button-row"><button className="primary" onClick={() => onNavigate("Events")}>Build an event</button><button onClick={() => onNavigate("Activity")}>Inspect activity</button></div></section>
    <div className="stat-grid"><Stat label="Product" value={bootstrap?.productVersion ?? "…"} /><Stat label="API" value={bootstrap ? `v${bootstrap.apiVersion}` : "…"} /><Stat label="History" value={bootstrap?.workspace.historyEnabled ? "Enabled" : "Disabled"} /><Stat label="Signing" value={bootstrap?.key.matchingPrivateKey ? "Key matched" : "Check key"} /></div>
    <Section title="Recent activity" hint="Persistent delivery attempts"><ActivityTable items={bootstrap?.recentActivity ?? []} /></Section>
  </div>;
}

function EventBuilder({ initialScenarioID }: { initialScenarioID?: string }) {
  const queryClient = useQueryClient();
  const [view, setView] = useState<EditorView>("form");
  const [selectedID, setSelectedID] = useState(initialScenarioID ?? "");
  const [draft, setDraft] = useState<JSONObject>({});
  const [raw, setRaw] = useState("{}");
  const [rawError, setRawError] = useState("");
  const [destinationURL, setDestinationURL] = useState("");
  const [preview, setPreview] = useState("");
  const [result, setResult] = useState<DeliveryResult>();
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const validation = useRef(0);
  const scenariosQuery = useQuery({ queryKey: ["scenarios", "valid"], queryFn: async () => successful<{ items: ScenarioSummary[] | null }>(await listScenarios()).items?.filter((item) => item.valid !== false) ?? [] });
  useEffect(() => { if (!selectedID && scenariosQuery.data?.[0]) setSelectedID(scenariosQuery.data[0].id); }, [selectedID, scenariosQuery.data]);
  const scenarioQuery = useQuery({ queryKey: ["scenario", selectedID], enabled: Boolean(selectedID), queryFn: async () => successful<ScenarioDetail>(await getScenario({ id: selectedID })) });
  useEffect(() => { if (scenarioQuery.data) { const value = scenarioQuery.data.draftPayload as JSONObject; setDraft(value); setRaw(JSON.stringify(value, null, 2)); setRawError(""); setPreview(""); setResult(undefined); } }, [scenarioQuery.data]);
  const applyDraft = (next: JSONObject) => { setDraft(next); setRaw(JSON.stringify(next, null, 2)); setRawError(""); setPreview(""); };
  const parseRaw = async (value: string) => {
    const currentValidation = ++validation.current;
    setRaw(value); setPreview("");
    try {
      const parsed: unknown = JSON.parse(value);
      if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") throw new Error("Payload must be a JSON object");
      const scenario = scenarioQuery.data; if (!scenario) return;
      successful(await validateEvent({ eventType: scenario.eventType, eventVersion: scenario.eventVersion, payload: parsed as JSONObject }));
      if (currentValidation === validation.current) { setDraft(parsed as JSONObject); setRawError(""); }
    } catch (error) { if (currentValidation === validation.current) setRawError(error instanceof Error ? error.message : String(error)); }
  };
  const request = () => ({ eventType: scenarioQuery.data!.eventType, eventVersion: scenarioQuery.data!.eventVersion, payload: draft, destination: scenarioQuery.data!.destination, destinationUrl: destinationURL || undefined });
  const createPreview = async () => {
    if (!scenarioQuery.data || rawError) return; setBusy(true); setMessage("");
    try { const generated = successful<{ rawHttp: string }>(await generateEvent(request())); setPreview(generated.rawHttp); }
    catch (error) { setMessage(error instanceof Error ? error.message : String(error)); }
    finally { setBusy(false); }
  };
  useEffect(() => { if (view === "http" && !preview) void createPreview(); }, [view]);
  const send = async () => {
    if (!scenarioQuery.data || rawError) return; setBusy(true); setMessage("");
    try { const delivery = successful<DeliveryResult>(await triggerEvent(request())); setResult(delivery); setMessage(delivery.error || `${delivery.outcome} · HTTP ${delivery.status ?? "—"}`); await queryClient.invalidateQueries({ queryKey: ["activity"] }); }
    catch (error) { setMessage(error instanceof Error ? error.message : String(error)); }
    finally { setBusy(false); }
  };
  const saveCopy = async () => {
    if (!scenarioQuery.data || rawError) return;
    const targetID = window.prompt("New scenario ID", `${scenarioQuery.data.id}-copy`); if (!targetID) return;
    setBusy(true);
    try { const saved = successful<ScenarioDetail>(await duplicateScenario({ sourceId: scenarioQuery.data.id, targetId: targetID, payload: draft })); await queryClient.invalidateQueries({ queryKey: ["scenarios"] }); setSelectedID(saved.id); setMessage(`Saved ${saved.id}`); }
    catch (error) { setMessage(error instanceof Error ? error.message : String(error)); }
    finally { setBusy(false); }
  };
  const scenario = scenarioQuery.data;
  return <>
    <section className="context-bar"><div><small>Scenario</small><select value={selectedID} onChange={(event) => setSelectedID(event.target.value)}>{scenariosQuery.data?.map((item) => <option value={item.id} key={item.id}>{item.name}</option>)}</select></div><div><small>Event</small><strong>{scenario ? `${scenario.eventType}@${scenario.eventVersion}` : "Loading…"}</strong></div><div><small>Signing</small><strong>Simulator RSA key</strong></div></section>
    <section className="builder-layout">
      <div className="editor-card"><div className="tabs" role="tablist">{(["form", "json", "http"] as const).map((item) => <button aria-selected={view === item} className={view === item ? "tab active" : "tab"} key={item} onClick={() => setView(item)} role="tab">{item === "form" ? "Form" : item === "json" ? "Raw JSON Payload" : "Raw HTTP Preview"}</button>)}</div>
        {view === "form" && <ObjectEditor value={draft} onChange={applyDraft} />}
        {view === "json" && <div className="raw-editor"><textarea aria-label="Raw JSON payload" value={raw} onChange={(event) => void parseRaw(event.target.value)} spellCheck={false} />{rawError && <Notice tone="error">{rawError}</Notice>}</div>}
        {view === "http" && <pre className="code-preview">{preview || (busy ? "Generating signed preview…" : "Preview unavailable")}</pre>}
      </div>
      <aside className="run-panel"><p className="eyebrow">Local delivery</p><h2>Send a signed event</h2><p>Run-local edits never change the source scenario.</p><label>Temporary loopback URL<input placeholder="Use configured destination" value={destinationURL} onChange={(event) => setDestinationURL(event.target.value)} /></label><dl><div><dt>Expected status</dt><dd>{scenario?.expectedStatuses?.join(", ") || "2xx"}</dd></div><div><dt>History</dt><dd>Persistent</dd></div><div><dt>Source</dt><dd>{scenario?.builtIn ? "Built-in" : "Custom"}</dd></div></dl><button className="send-button" disabled={busy || Boolean(rawError) || !scenario} onClick={() => void send()}>Send event <span>↗</span></button><button className="secondary-wide" disabled={busy || Boolean(rawError)} onClick={() => void saveCopy()}>Save as copy</button>{message && <Notice tone={result?.error ? "error" : "success"}>{message}</Notice>}</aside>
    </section>
  </>;
}

function ObjectEditor({ value, onChange }: { value: JSONObject; onChange: (value: JSONObject) => void }) { return <div className="form-content">{Object.entries(value).map(([key, item]) => <ValueEditor key={key} label={key} value={item} onChange={(next) => onChange({ ...value, [key]: next })} />)}</div>; }
function ValueEditor({ label, value, onChange }: { label: string; value: unknown; onChange: (value: unknown) => void }) {
  if (value !== null && !Array.isArray(value) && typeof value === "object") return <fieldset><legend>{friendly(label)}</legend><div className="nested-fields">{Object.entries(value as JSONObject).map(([key, item]) => <ValueEditor key={key} label={key} value={item} onChange={(next) => onChange({ ...(value as JSONObject), [key]: next })} />)}</div></fieldset>;
  if (typeof value === "boolean") return <label className="check-field"><input type="checkbox" checked={value} onChange={(event) => onChange(event.target.checked)} />{friendly(label)}</label>;
  if (Array.isArray(value) || value === null) return <label>{friendly(label)}<textarea rows={Math.min(8, Math.max(2, JSON.stringify(value, null, 2).split("\n").length))} value={JSON.stringify(value, null, 2)} onChange={(event) => { try { onChange(JSON.parse(event.target.value)); } catch { /* keep last semantic value */ } }} /></label>;
  return <label>{friendly(label)}<input value={String(value)} type={typeof value === "number" ? "number" : "text"} onChange={(event) => onChange(typeof value === "number" ? Number(event.target.value) : event.target.value)} /></label>;
}

function Scenarios({ onOpen }: { onOpen: (id: string) => void }) {
  const queryClient = useQueryClient();
  const [editingID, setEditingID] = useState("");
  const query = useQuery({ queryKey: ["scenarios"], queryFn: async () => successful<{ items: ScenarioSummary[] | null }>(await listScenarios()).items ?? [], refetchInterval: 1500 });
  const duplicate = async (item: ScenarioSummary) => { const id = window.prompt("New scenario ID", `${item.id}-copy`); if (!id) return; const detail = successful<ScenarioDetail>(await getScenario({ id: item.id })); successful(await duplicateScenario({ sourceId: item.id, targetId: id, payload: detail.draftPayload as JSONObject })); await queryClient.invalidateQueries({ queryKey: ["scenarios"] }); };
  return <div className="page-content">
    <Section title="Scenario library" hint="Built-ins remain immutable; custom source files stay authoritative">
      <div className="card-grid">{query.data?.map((item) => <article className="item-card" key={item.id}>
        <span className={`badge ${item.valid === false ? "invalid" : ""}`}>{item.valid === false ? "Invalid source" : item.builtIn ? "Built-in" : "Custom"}</span>
        <h3>{item.name || item.id}</h3><p>{item.description || item.validationErrors?.join("; ")}</p>
        <code>{item.eventType ? `${item.eventType}@${item.eventVersion}` : item.sourceFormat}</code>
        <div className="button-row"><button className="primary" disabled={item.valid === false} onClick={() => onOpen(item.id)}>Open</button><button disabled={item.valid === false} onClick={() => void duplicate(item)}>Duplicate</button>{!item.builtIn && <button onClick={() => setEditingID(item.id)}>Edit source</button>}</div>
      </article>)}</div>
    </Section>
    {editingID && <ScenarioSourceEditor id={editingID} onClose={() => setEditingID("")} onSelect={setEditingID} />}
  </div>;
}

function ScenarioSourceEditor({ id, onClose, onSelect }: { id: string; onClose: () => void; onSelect: (id: string) => void }) {
  const queryClient = useQueryClient();
  const query = useQuery({ queryKey: ["scenario-source", id], queryFn: async () => successful<ScenarioDetail>(await getScenario({ id })), refetchInterval: 1500 });
  const loadedID = useRef("");
  const [draftSource, setDraftSource] = useState("");
  const [baselineRevision, setBaselineRevision] = useState("");
  const [ignoredRevision, setIgnoredRevision] = useState("");
  const [diskChange, setDiskChange] = useState<ScenarioDetail>();
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const current = query.data;
    if (!current) return;
    if (loadedID.current !== id) {
      loadedID.current = id;
      setDraftSource(current.source);
      setBaselineRevision(current.revision);
      setIgnoredRevision("");
      setDiskChange(undefined);
      setMessage("");
      return;
    }
    if (baselineRevision && current.revision !== baselineRevision && current.revision !== ignoredRevision) setDiskChange(current);
  }, [id, query.data, baselineRevision, ignoredRevision]);

  const accept = (saved: ScenarioDetail) => {
    loadedID.current = saved.id;
    setDraftSource(saved.source);
    setBaselineRevision(saved.revision);
    setIgnoredRevision("");
    setDiskChange(undefined);
    queryClient.setQueryData(["scenario-source", saved.id], saved);
  };
  const save = async () => {
    setBusy(true); setMessage("");
    try {
      const response = await updateScenarioSource({ id, revision: baselineRevision, source: draftSource });
      if (response.status !== 200) {
        if (response.status === 409) await query.refetch();
        throw new Error(failureMessage(response.data, `Save failed with HTTP ${response.status}`));
      }
      const saved = response.data as ScenarioDetail; accept(saved); setMessage("Source saved"); await queryClient.invalidateQueries({ queryKey: ["scenarios"] });
    } catch (error) { setMessage(error instanceof Error ? error.message : String(error)); }
    finally { setBusy(false); }
  };
  const saveCopy = async () => {
    const targetID = window.prompt("New scenario ID", `${id}-copy`); if (!targetID) return;
    setBusy(true); setMessage("");
    try {
      const response = await saveScenarioSourceCopy({ sourceId: id, targetId: targetID, source: draftSource });
      const saved = successful<ScenarioDetail>(response); await queryClient.invalidateQueries({ queryKey: ["scenarios"] }); onSelect(saved.id); setMessage(`Saved ${saved.id}`);
    } catch (error) { setMessage(error instanceof Error ? error.message : String(error)); }
    finally { setBusy(false); }
  };
  const reload = () => {
    if (!diskChange) return;
    accept(diskChange); setMessage(diskChange.valid ? "Reloaded file from disk" : "Reloaded invalid file for repair");
  };
  const keepDraft = () => { if (diskChange) setIgnoredRevision(diskChange.revision); setDiskChange(undefined); setMessage("Kept your draft; saving to the original file will still require its prior revision"); };
  const detail = query.data;
  const ready = loadedID.current === id && Boolean(baselineRevision);
  return <Section title={`Edit source · ${id}`} hint={`${detail?.sourceFormat?.toUpperCase() ?? "Source"} · exact file text`}>
    {query.isError && <Notice tone="error">Source file is no longer available at this scenario ID. Your draft is still open and can be saved as a copy.</Notice>}
    {diskChange && <div className="source-conflict" role="alert"><strong>File changed on disk</strong><p>Your open draft was not replaced.</p><div className="button-row"><button className="primary" onClick={reload}>Reload file</button><button onClick={keepDraft}>Keep draft</button></div></div>}
    {detail?.valid === false && detail.validationErrors?.map((problem) => <Notice tone="error" key={problem}>{problem}</Notice>)}
    {ready ? <><textarea className="source-editor" aria-label="Scenario source" value={draftSource} onChange={(event) => { setDraftSource(event.target.value); setMessage(""); }} spellCheck={false} />
      <div className="source-actions"><div className="button-row"><button className="primary" disabled={busy || draftSource === detail?.source} onClick={() => void save()}>Save source</button><button disabled={busy || !draftSource} onClick={() => void saveCopy()}>Save as copy</button><button onClick={onClose}>Close</button></div><code>{baselineRevision}</code></div></> : <p className="empty">Loading source…</p>}
    {message && <Notice tone={message.includes("saved") || message.includes("Reloaded") ? "success" : "error"}>{message}</Notice>}
  </Section>;
}

function ActivityPage() { const [selected, setSelected] = useState<string>(); const query = useQuery({ queryKey: ["activity"], queryFn: async () => successful<{ items: Activity[] | null }>(await listRuns({ limit: 100, offset: 0 })).items ?? [] }); return <div className="page-content activity-layout"><Section title="Delivery activity" hint="Newest attempts first"><ActivityTable items={query.data ?? []} onSelect={setSelected} selected={selected} /></Section>{selected && <AttemptInspector id={selected} />}</div>; }
function AttemptInspector({ id }: { id: string }) {
  const queryClient = useQueryClient(); const query = useQuery({ queryKey: ["attempt", id], queryFn: async () => successful<DeliveryAttemptDetail>(await getDeliveryAttempt(id)) });
  const replay = async (mode: "exact" | "regenerated") => { successful(await replayDeliveryAttempt(id, { mode })); await queryClient.invalidateQueries({ queryKey: ["activity"] }); };
  const detail = query.data;
  return <Section title="Attempt inspector" hint={id}>{detail ? <div className="inspector"><Info label="Destination" value={`${detail.attempt.method} ${detail.attempt.url}`} /><Info label="Outcome" value={`${detail.attempt.outcome} · ${detail.attempt.responseStatus ?? "—"} · ${detail.attempt.durationMs.toFixed(2)}ms`} /><h3>Request headers</h3><Code value={detail.attempt.requestHeaders} /><h3>Exact body</h3><Code value={detail.event.rawBody} /><h3>Signature input</h3><Code value={detail.signatureInput} /><h3>Response</h3><Code value={{ headers: detail.attempt.responseHeaders, body: detail.attempt.responseBody, truncated: detail.attempt.responseTruncated }} /><div className="button-row"><button className="primary" onClick={() => void replay("exact")}>Replay exact</button><button onClick={() => void replay("regenerated")}>Replay regenerated</button></div></div> : <p>Loading attempt…</p>}</Section>;
}

function Keys() {
  const queryClient = useQueryClient(); const key = useQuery({ queryKey: ["key"], queryFn: async () => successful<KeyInfo>(await getSimulatorKeyInfo()) }); const publicKey = useQuery({ queryKey: ["public-key"], queryFn: async () => successful<{ path: string; pem: string }>(await getSimulatorPublicKey()) });
  const rotate = async () => { if (!window.confirm("Rotate the simulator key pair? Existing receiver setups will need the new public key.")) return; successful(await rotateSimulatorKey({ headers: { "Content-Type": "application/json" } })); await queryClient.invalidateQueries({ queryKey: ["key"] }); await queryClient.invalidateQueries({ queryKey: ["public-key"] }); };
  return <div className="page-content"><Section title="Simulator signing key" hint="The private key stays in the local workspace"><div className="detail-grid"><Info label="Algorithm" value={key.data ? `${key.data.algorithm} ${key.data.bits}` : "…"} /><Info label="Fingerprint" value={key.data?.fingerprint ?? "…"} /><Info label="Pair status" value={key.data?.matchingPrivateKey ? "Public/private keys match" : "Key mismatch"} /><Info label="Public key path" value={key.data?.path ?? "…"} /></div><Code value={publicKey.data?.pem ?? "Loading public key…"} /><button className="danger" onClick={() => void rotate()}>Rotate key pair</button></Section></div>;
}

function Destinations({ bootstrap }: { bootstrap?: Bootstrap }) { return <div className="page-content"><Section title="Destination" hint="Delivery is restricted to loopback receivers"><div className="detail-grid"><Info label="Name" value={bootstrap?.workspace.defaultDestination ?? "…"} /><Info label="URL" value={bootstrap?.workspace.destinationUrl ?? "…"} /></div><Notice>Temporary URLs can be supplied from the event builder and are never saved.</Notice></Section></div>; }
function Settings({ bootstrap }: { bootstrap?: Bootstrap }) { return <div className="page-content"><Section title="Runtime settings" hint="Read-only workspace summary"><div className="detail-grid"><Info label="Workspace" value={bootstrap?.workspace.path ?? "…"} /><Info label="Database" value={bootstrap?.workspace.databasePath ?? "…"} /><Info label="History" value={bootstrap?.workspace.historyEnabled ? "Enabled" : "Disabled"} /><Info label="Capabilities" value={bootstrap?.capabilities?.join(", ") ?? "…"} /></div></Section></div>; }
function APIDocs() { return <div className="page-content"><Section title="Local API" hint="OpenAPI 3.1, generated from the Go contracts"><p>Use the committed document for client generation or inspect the live document exposed by this process.</p><div className="button-row"><a className="button primary" href="/api/openapi" target="_blank">Open live OpenAPI</a><a className="button" href="/api/capabilities" target="_blank">View capabilities</a></div></Section></div>; }
function ActivityTable({ items, onSelect, selected }: { items: Activity[]; onSelect?: (id: string) => void; selected?: string }) { return items.length ? <div className="activity-table">{items.map((item) => <button className={selected === item.attemptId ? "activity-row selected" : "activity-row"} key={item.attemptId} onClick={() => onSelect?.(item.attemptId)} disabled={!onSelect}><span><strong>{item.eventType}@{item.eventVersion}</strong><small>{new Date(item.createdAt).toLocaleString()}</small></span><span className={`outcome ${item.outcome}`}>{item.outcome}</span><code>{item.status ?? "—"}</code><code>{item.durationMs.toFixed(1)}ms</code></button>)}</div> : <p className="empty">No retained delivery activity yet.</p>; }
function Section({ title, hint, children }: { title: string; hint?: string; children: React.ReactNode }) { return <section className="section-card"><div className="section-title"><div><h2>{title}</h2>{hint && <p>{hint}</p>}</div></div>{children}</section>; }
function Stat({ label, value }: { label: string; value: string }) { return <div className="stat"><small>{label}</small><strong>{value}</strong></div>; }
function Info({ label, value }: { label: string; value: string }) { return <div className="info"><small>{label}</small><strong>{value}</strong></div>; }
function Notice({ children, tone = "info" }: { children: React.ReactNode; tone?: "info" | "error" | "success" }) { return <p className={`notice ${tone}`}>{children}</p>; }
function Code({ value }: { value: unknown }) { const text = typeof value === "string" ? value : JSON.stringify(value, null, 2); return <pre className="code-block">{text}</pre>; }
function friendly(value: string) { return value.replaceAll("_", " ").replace(/\b\w/g, (letter) => letter.toUpperCase()); }
