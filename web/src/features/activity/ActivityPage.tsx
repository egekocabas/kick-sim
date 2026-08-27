import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { getDeliveryAttempt, listRuns, replayDeliveryAttempt } from "../../api/generated/client";
import type { Activity, DeliveryAttemptDetail } from "../../api/generated/models";
import { ActivityTable, Code, Info, Notice, QueryState, Section } from "../../components/ui";
import { errorMessage, successful } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";

export function ActivityPage() {
  const [selected, setSelected] = useState<string>();
  const query = useQuery({
    queryKey: queryKeys.activity,
    queryFn: async () =>
      successful<{ items: Activity[] | null }>(await listRuns({ limit: 100, offset: 0 })).items ?? [],
  });
  return (
    <div className="page-content activity-layout">
      <Section title="Delivery activity" hint="Newest attempts first">
        <QueryState pending={query.isPending} error={query.error} empty={query.data?.length === 0} />
        {query.data && <ActivityTable items={query.data} onSelect={setSelected} selected={selected} />}
      </Section>
      {selected && <AttemptInspector id={selected} />}
    </div>
  );
}

function AttemptInspector({ id }: { id: string }) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: queryKeys.attempt(id),
    queryFn: async () => successful<DeliveryAttemptDetail>(await getDeliveryAttempt(id)),
  });
  const replay = useMutation({
    mutationFn: async (mode: "exact" | "regenerated") => successful(await replayDeliveryAttempt(id, { mode })),
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: queryKeys.activity }),
  });
  const detail = query.data;
  return (
    <Section title="Attempt inspector" hint={id}>
      <QueryState pending={query.isPending} error={query.error} />
      {detail && (
        <div className="inspector">
          <Info label="Destination" value={`${detail.attempt.method} ${detail.attempt.url}`} />
          <Info
            label="Outcome"
            value={`${detail.attempt.outcome} · ${detail.attempt.responseStatus ?? "—"} · ${detail.attempt.durationMs.toFixed(2)}ms`}
          />
          <h3>Request headers</h3>
          <Code value={detail.attempt.requestHeaders} />
          <h3>Exact body</h3>
          <Code value={detail.event.rawBody} />
          <h3>Signature input</h3>
          <Code value={detail.signatureInput} />
          <h3>Response</h3>
          <Code
            value={{
              headers: detail.attempt.responseHeaders,
              body: detail.attempt.responseBody,
              truncated: detail.attempt.responseTruncated,
            }}
          />
          <div className="button-row">
            <button
              className="primary"
              disabled={replay.isPending}
              onClick={() => replay.mutate("exact")}
              type="button"
            >
              Replay exact
            </button>
            <button disabled={replay.isPending} onClick={() => replay.mutate("regenerated")} type="button">
              Replay regenerated
            </button>
          </div>
        </div>
      )}
      {replay.error && <Notice tone="error">{errorMessage(replay.error)}</Notice>}
    </Section>
  );
}
