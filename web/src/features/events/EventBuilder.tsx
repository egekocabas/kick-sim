import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { duplicateScenario, generateEvent, getScenario, listScenarios, triggerEvent, validateEvent } from "../../api/generated/client";
import type { DeliveryResult, EventDeliveryRequest, ScenarioDetail, ScenarioSummary } from "../../api/generated/models";
import { Notice, QueryState } from "../../components/ui";
import { errorMessage, successful } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";
import { ObjectEditor, type JSONObject } from "./PayloadEditor";

type EditorView = "form" | "json" | "http";

function useDebouncedValue<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(value), delay);
    return () => window.clearTimeout(timer);
  }, [delay, value]);
  return debounced;
}

function isJSONObject(value: unknown): value is JSONObject {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function EventBuilder({ initialScenarioID }: { initialScenarioID: string | undefined }) {
  const queryClient = useQueryClient();
  const [view, setView] = useState<EditorView>("form");
  const [selectedID, setSelectedID] = useState(initialScenarioID ?? "");
  const [draft, setDraft] = useState<JSONObject>({});
  const [raw, setRaw] = useState("{}");
  const [rawError, setRawError] = useState("");
  const [validatingRaw, setValidatingRaw] = useState(false);
  const [destinationURL, setDestinationURL] = useState("");
  const [result, setResult] = useState<DeliveryResult>();
  const [message, setMessage] = useState("");
  const validationSequence = useRef(0);
  const validationTimer = useRef<number | undefined>(undefined);

  const scenariosQuery = useQuery({
    queryKey: queryKeys.validScenarios,
    queryFn: async () => successful<{ items: ScenarioSummary[] | null }>(await listScenarios()).items?.filter((item) => item.valid !== false && item.kind !== "timeline") ?? [],
  });
  useEffect(() => {
    if (!selectedID && scenariosQuery.data?.[0]) setSelectedID(scenariosQuery.data[0].id);
  }, [selectedID, scenariosQuery.data]);

  const scenarioQuery = useQuery({
    queryKey: queryKeys.scenario(selectedID),
    enabled: Boolean(selectedID),
    queryFn: async () => successful<ScenarioDetail>(await getScenario({ id: selectedID })),
  });
  useEffect(() => {
    const value = scenarioQuery.data?.draftPayload;
    if (!isJSONObject(value)) return;
    validationSequence.current++;
    if (validationTimer.current !== undefined) window.clearTimeout(validationTimer.current);
    setDraft(value);
    setRaw(JSON.stringify(value, null, 2));
    setRawError("");
    setValidatingRaw(false);
    setResult(undefined);
    setMessage("");
  }, [scenarioQuery.data]);
  useEffect(() => () => {
    if (validationTimer.current !== undefined) window.clearTimeout(validationTimer.current);
  }, []);

  const applyDraft = (next: JSONObject) => {
    validationSequence.current++;
    if (validationTimer.current !== undefined) window.clearTimeout(validationTimer.current);
    setDraft(next);
    setRaw(JSON.stringify(next, null, 2));
    setRawError("");
    setValidatingRaw(false);
  };

  const applyRaw = (value: string) => {
    const currentValidation = ++validationSequence.current;
    if (validationTimer.current !== undefined) window.clearTimeout(validationTimer.current);
    setRaw(value);
    setResult(undefined);
    let parsed: unknown;
    try {
      parsed = JSON.parse(value);
      if (!isJSONObject(parsed)) throw new Error("Payload must be a JSON object");
    } catch (problem) {
      setRawError(errorMessage(problem));
      setValidatingRaw(false);
      return;
    }
    const scenario = scenarioQuery.data;
    if (!scenario) return;
    setRawError("");
    setValidatingRaw(true);
    validationTimer.current = window.setTimeout(async () => {
      try {
        successful(await validateEvent({ eventType: scenario.eventType, eventVersion: scenario.eventVersion, payload: parsed as JSONObject }));
        if (currentValidation === validationSequence.current) {
          setDraft(parsed as JSONObject);
          setRawError("");
          setValidatingRaw(false);
        }
      } catch (problem) {
        if (currentValidation === validationSequence.current) {
          setRawError(errorMessage(problem));
          setValidatingRaw(false);
        }
      }
    }, 300);
  };

  const scenario = scenarioQuery.data;
  const request = useMemo<EventDeliveryRequest | undefined>(() => {
    if (!scenario) return undefined;
    return {
      eventType: scenario.eventType,
      eventVersion: scenario.eventVersion,
      payload: draft,
      ...(scenario.destination ? { destination: scenario.destination } : {}),
      ...(destinationURL ? { destinationUrl: destinationURL } : {}),
    };
  }, [destinationURL, draft, scenario]);
  const previewRequest = useDebouncedValue(request, 300);
  const preview = useQuery({
    queryKey: queryKeys.preview(previewRequest),
    enabled: view === "http" && Boolean(previewRequest) && !rawError && !validatingRaw,
    queryFn: async () => {
      if (!previewRequest) throw new Error("Preview request is unavailable");
      return successful<{ rawHttp: string }>(await generateEvent(previewRequest)).rawHttp;
    },
  });

  const send = useMutation({
    mutationFn: async () => {
      if (!request) throw new Error("Select a valid scenario first");
      return successful<DeliveryResult>(await triggerEvent(request));
    },
    onSuccess: async (delivery) => {
      setResult(delivery);
      setMessage(delivery.error || `${delivery.outcome} · HTTP ${delivery.status ?? "—"}`);
      await queryClient.invalidateQueries({ queryKey: queryKeys.activity });
    },
  });
  const saveCopy = useMutation({
    mutationFn: async (targetID: string) => {
      if (!scenario) throw new Error("Select a valid scenario first");
      return successful<ScenarioDetail>(await duplicateScenario({ sourceId: scenario.id, targetId: targetID, payload: draft }));
    },
    onSuccess: async (saved) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.scenarios });
      setSelectedID(saved.id);
      setMessage(`Saved ${saved.id}`);
    },
  });
  const busy = send.isPending || saveCopy.isPending;
  const mutationError = send.error ?? saveCopy.error;

  return (
    <>
      <section className="context-bar">
        <div><small>Scenario</small><select aria-label="Scenario" value={selectedID} onChange={(event) => setSelectedID(event.target.value)}>{scenariosQuery.data?.map((item) => <option value={item.id} key={item.id}>{item.name}</option>)}</select></div>
        <div><small>Event</small><strong>{scenario ? `${scenario.eventType}@${scenario.eventVersion}` : "Loading…"}</strong></div>
        <div><small>Signing</small><strong>Simulator RSA key</strong></div>
      </section>
      <QueryState pending={scenariosQuery.isPending || scenarioQuery.isPending} error={scenariosQuery.error ?? scenarioQuery.error} />
      <section className="builder-layout">
        <div className="editor-card">
          <div className="tabs" role="tablist" aria-label="Payload editor view">
            {(["form", "json", "http"] as const).map((item) => (
              <button
                id={`editor-tab-${item}`}
                aria-controls={`editor-panel-${item}`}
                aria-selected={view === item}
                className={view === item ? "tab active" : "tab"}
                key={item}
                onClick={() => setView(item)}
                role="tab"
                type="button"
              >
                {item === "form" ? "Form" : item === "json" ? "Raw JSON Payload" : "Raw HTTP Preview"}
              </button>
            ))}
          </div>
          {view === "form" && <div id="editor-panel-form" role="tabpanel" aria-labelledby="editor-tab-form"><ObjectEditor value={draft} onChange={applyDraft} /></div>}
          {view === "json" && <div id="editor-panel-json" role="tabpanel" aria-labelledby="editor-tab-json" className="raw-editor"><textarea aria-label="Raw JSON payload" value={raw} onChange={(event) => applyRaw(event.target.value)} spellCheck={false} />{validatingRaw && <p role="status">Validating payload…</p>}{rawError && <Notice tone="error">{rawError}</Notice>}</div>}
          {view === "http" && <div id="editor-panel-http" role="tabpanel" aria-labelledby="editor-tab-http">{preview.isError && <Notice tone="error">{preview.error.message}</Notice>}<pre className="code-preview">{preview.data ?? (preview.isFetching ? "Generating signed preview…" : "Preview unavailable")}</pre></div>}
        </div>
        <aside className="run-panel">
          <p className="eyebrow">Local delivery</p><h2>Send a signed event</h2><p>Run-local edits never change the source scenario.</p>
          <label>Temporary loopback URL<input placeholder="Use configured destination" value={destinationURL} onChange={(event) => setDestinationURL(event.target.value)} /></label>
          <dl><div><dt>Expected status</dt><dd>{scenario?.expectedStatuses?.join(", ") || "2xx"}</dd></div><div><dt>History</dt><dd>Persistent</dd></div><div><dt>Source</dt><dd>{scenario?.builtIn ? "Built-in" : "Custom"}</dd></div></dl>
          <button className="send-button" disabled={busy || Boolean(rawError) || validatingRaw || !scenario} onClick={() => { setMessage(""); send.mutate(); }} type="button">Send event <span>↗</span></button>
          <button className="secondary-wide" disabled={busy || Boolean(rawError) || validatingRaw || !scenario} onClick={() => { const targetID = scenario ? window.prompt("New scenario ID", `${scenario.id}-copy`) : null; if (targetID) saveCopy.mutate(targetID); }} type="button">Save as copy</button>
          {mutationError && <Notice tone="error">{errorMessage(mutationError)}</Notice>}
          {message && <Notice tone={result?.error ? "error" : "success"}>{message}</Notice>}
        </aside>
      </section>
    </>
  );
}
