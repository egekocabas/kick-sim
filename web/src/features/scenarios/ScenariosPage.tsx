import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import {
  duplicateScenario,
  getScenario,
  listScenarios,
  saveScenarioSourceCopy,
  updateScenarioSource,
} from "../../api/generated/client";
import type { ScenarioDetail, ScenarioSummary } from "../../api/generated/models";
import { ScenarioCopyDialog, type ScenarioCopyValues } from "../../components/ScenarioCopyDialog";
import { Notice, QueryState, Section } from "../../components/ui";
import { errorMessage, failureMessage, successful } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";

type JSONObject = Record<string, unknown>;

export function ScenariosPage({ onOpen }: { onOpen: (id: string) => void }) {
  const queryClient = useQueryClient();
  const [copySource, setCopySource] = useState<ScenarioSummary>();
  const [editingID, setEditingID] = useState("");
  const query = useQuery({
    queryKey: queryKeys.scenarios,
    queryFn: async () =>
      (successful<{ items: ScenarioSummary[] | null }>(await listScenarios()).items ?? []).filter(
        (item) => item.kind !== "timeline",
      ),
    refetchInterval: 1_500,
  });
  const duplicate = useMutation({
    mutationFn: async ({ item, values }: { item: ScenarioSummary; values: ScenarioCopyValues }) => {
      const detail = successful<ScenarioDetail>(await getScenario({ id: item.id }));
      return successful<ScenarioDetail>(
        await duplicateScenario({ sourceId: item.id, ...values, payload: detail.draftPayload as JSONObject }),
      );
    },
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: queryKeys.scenarios }),
  });

  return (
    <div className="page-content">
      <Section title="Scenario library" hint="Built-ins remain immutable; custom source files stay authoritative">
        <QueryState pending={query.isPending} error={query.error} empty={query.data?.length === 0} />
        <div className="card-grid">
          {query.data?.map((item) => (
            <article className="item-card" key={item.id}>
              <span className={`badge ${item.valid === false ? "invalid" : ""}`}>
                {item.valid === false ? "Invalid source" : item.builtIn ? "Built-in" : "Custom"}
              </span>
              <h3>{item.name || item.id}</h3>
              <p>{item.description || item.validationErrors?.join("; ")}</p>
              <code>{item.eventType ? `${item.eventType}@${item.eventVersion}` : item.sourceFormat}</code>
              <div className="button-row">
                <button
                  className="primary"
                  disabled={item.valid === false}
                  onClick={() => onOpen(item.id)}
                  type="button"
                >
                  Open
                </button>
                <button
                  disabled={item.valid === false || duplicate.isPending}
                  onClick={() => setCopySource(item)}
                  type="button"
                >
                  Duplicate
                </button>
                {!item.builtIn && (
                  <button onClick={() => setEditingID(item.id)} type="button">
                    Edit source
                  </button>
                )}
              </div>
            </article>
          ))}
        </div>
        {duplicate.error && <Notice tone="error">{errorMessage(duplicate.error)}</Notice>}
      </Section>
      {copySource && (
        <ScenarioCopyDialog
          source={copySource}
          onSave={(values) => duplicate.mutateAsync({ item: copySource, values })}
          onClose={() => setCopySource(undefined)}
        />
      )}
      {editingID && <ScenarioSourceEditor id={editingID} onClose={() => setEditingID("")} onSelect={setEditingID} />}
    </div>
  );
}

function ScenarioSourceEditor({
  id,
  onClose,
  onSelect,
}: {
  id: string;
  onClose: () => void;
  onSelect: (id: string) => void;
}) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: queryKeys.scenarioSource(id),
    queryFn: async () => successful<ScenarioDetail>(await getScenario({ id })),
    refetchInterval: 1_500,
  });
  const loadedID = useRef("");
  const [copyOpen, setCopyOpen] = useState(false);
  const [draftSource, setDraftSource] = useState("");
  const [baselineRevision, setBaselineRevision] = useState("");
  const [ignoredRevision, setIgnoredRevision] = useState("");
  const [diskChange, setDiskChange] = useState<ScenarioDetail>();
  const [message, setMessage] = useState("");

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
    if (baselineRevision && current.revision !== baselineRevision && current.revision !== ignoredRevision) {
      setDiskChange(current);
    }
  }, [baselineRevision, id, ignoredRevision, query.data]);

  const accept = (saved: ScenarioDetail) => {
    loadedID.current = saved.id;
    setDraftSource(saved.source);
    setBaselineRevision(saved.revision);
    setIgnoredRevision("");
    setDiskChange(undefined);
    queryClient.setQueryData(queryKeys.scenarioSource(saved.id), saved);
  };
  const save = useMutation({
    mutationFn: async () => {
      const response = await updateScenarioSource({ id, revision: baselineRevision, source: draftSource });
      if (response.status !== 200) {
        if (response.status === 409) await query.refetch();
        throw new Error(failureMessage(response.data, `Save failed with HTTP ${response.status}`));
      }
      return response.data as ScenarioDetail;
    },
    onSuccess: async (saved) => {
      accept(saved);
      setMessage("Source saved");
      await queryClient.invalidateQueries({ queryKey: queryKeys.scenarios });
    },
  });
  const saveCopy = useMutation({
    mutationFn: async (values: ScenarioCopyValues) =>
      successful<ScenarioDetail>(await saveScenarioSourceCopy({ sourceId: id, ...values, source: draftSource })),
    onSuccess: async (saved) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.scenarios });
      onSelect(saved.id);
      setMessage(`Saved ${saved.id}`);
    },
  });
  const busy = save.isPending || saveCopy.isPending;
  const mutationError = save.error ?? saveCopy.error;
  const detail = query.data;
  const ready = loadedID.current === id && Boolean(baselineRevision);

  return (
    <Section
      title={`Edit source · ${id}`}
      hint={`${detail?.sourceFormat?.toUpperCase() ?? "Source"} · exact file text`}
    >
      {copyOpen && (
        <ScenarioCopyDialog
          source={{ id, name: detail?.name ?? id }}
          onSave={saveCopy.mutateAsync}
          onClose={() => setCopyOpen(false)}
        />
      )}
      {query.isError && (
        <Notice tone="error">
          Source file is no longer available at this scenario ID. Your draft is still open and can be saved as a copy.
        </Notice>
      )}
      {diskChange && (
        <div className="source-conflict" role="alert">
          <strong>File changed on disk</strong>
          <p>Your open draft was not replaced.</p>
          <div className="button-row">
            <button
              className="primary"
              onClick={() => {
                accept(diskChange);
                setMessage(diskChange.valid ? "Reloaded file from disk" : "Reloaded invalid file for repair");
              }}
              type="button"
            >
              Reload file
            </button>
            <button
              onClick={() => {
                setIgnoredRevision(diskChange.revision);
                setDiskChange(undefined);
                setMessage("Kept your draft; saving to the original file will still require its prior revision");
              }}
              type="button"
            >
              Keep draft
            </button>
          </div>
        </div>
      )}
      {detail?.valid === false &&
        detail.validationErrors?.map((problem) => (
          <Notice tone="error" key={problem}>
            {problem}
          </Notice>
        ))}
      {ready ? (
        <>
          <textarea
            className="source-editor"
            aria-label="Scenario source"
            value={draftSource}
            onChange={(event) => {
              setDraftSource(event.target.value);
              setMessage("");
            }}
            spellCheck={false}
          />
          <div className="source-actions">
            <div className="button-row">
              <button
                className="primary"
                disabled={busy || draftSource === detail?.source}
                onClick={() => {
                  setMessage("");
                  save.mutate();
                }}
                type="button"
              >
                Save source
              </button>
              <button disabled={busy || !draftSource} onClick={() => setCopyOpen(true)} type="button">
                Save as copy
              </button>
              <button onClick={onClose} type="button">
                Close
              </button>
            </div>
            <code>{baselineRevision}</code>
          </div>
        </>
      ) : (
        <p className="empty" role="status">
          Loading source…
        </p>
      )}
      {mutationError && <Notice tone="error">{errorMessage(mutationError)}</Notice>}
      {message && (
        <Notice tone={message.toLowerCase().includes("saved") || message.includes("Reloaded") ? "success" : "error"}>
          {message}
        </Notice>
      )}
    </Section>
  );
}
