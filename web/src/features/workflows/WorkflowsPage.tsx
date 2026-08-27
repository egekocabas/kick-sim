import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { listScenarios, listSuites, runSuite, runWorkflow } from "../../api/generated/client";
import type { ScenarioSummary, SuiteResult, SuiteSummary, WorkflowResult } from "../../api/generated/models";
import { Code, Notice, QueryState, Section } from "../../components/ui";
import { errorMessage, successful } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";

export function WorkflowsPage() {
  const queryClient = useQueryClient();
  const [message, setMessage] = useState("");
  const [report, setReport] = useState<WorkflowResult | SuiteResult>();
  const timelines = useQuery({
    queryKey: queryKeys.timelineScenarios,
    queryFn: async () =>
      (successful<{ items: ScenarioSummary[] | null }>(await listScenarios()).items ?? []).filter(
        (item) => item.kind === "timeline",
      ),
  });
  const suites = useQuery({
    queryKey: queryKeys.suites,
    queryFn: async () => successful<{ items: SuiteSummary[] | null }>(await listSuites()).items ?? [],
  });
  const timelineRun = useMutation({
    mutationFn: async (id: string) => successful<WorkflowResult>(await runWorkflow({ scenarioId: id })),
    onSuccess: async (value) => {
      setReport(value);
      setMessage(`${value.deliveries?.length ?? 0} deliveries passed`);
      await queryClient.invalidateQueries({ queryKey: queryKeys.activity });
    },
  });
  const suiteRun = useMutation({
    mutationFn: async (id: string) => successful<SuiteResult>(await runSuite({ suiteId: id })),
    onSuccess: async (value) => {
      setReport(value);
      setMessage(`${value.passedCases} passed, ${value.failedCases} failed`);
      await queryClient.invalidateQueries({ queryKey: queryKeys.activity });
    },
  });
  const busyID = timelineRun.isPending ? timelineRun.variables : suiteRun.isPending ? suiteRun.variables : "";
  const mutationError = timelineRun.error ?? suiteRun.error;

  return (
    <div className="page-content">
      <Section
        title="Timeline scenarios"
        hint="Multi-step workflows use deterministic logical time and retain every delivery attempt"
      >
        <QueryState pending={timelines.isPending} error={timelines.error} empty={timelines.data?.length === 0} />
        <div className="card-grid">
          {timelines.data?.map((item) => (
            <article className="item-card" key={item.id}>
              <span className="badge">{item.builtIn ? "Built-in" : "Custom"}</span>
              <h3>{item.name}</h3>
              <p>{item.description}</p>
              <code>{item.id}</code>
              <div className="button-row">
                <button
                  className="primary"
                  disabled={!item.valid || timelineRun.isPending || suiteRun.isPending}
                  onClick={() => {
                    setMessage("");
                    setReport(undefined);
                    timelineRun.mutate(item.id);
                  }}
                  type="button"
                >
                  {busyID === item.id ? "Running…" : "Run timeline"}
                </button>
              </div>
            </article>
          ))}
        </div>
      </Section>
      <Section title="Scenario suites" hint="Thresholds produce deterministic pass/fail results for local runs and CI">
        <QueryState pending={suites.isPending} error={suites.error} empty={suites.data?.length === 0} />
        <div className="card-grid">
          {suites.data?.map((item) => (
            <article className="item-card" key={item.id}>
              <span className={`badge ${!item.valid ? "invalid" : ""}`}>{item.builtIn ? "Built-in" : "Custom"}</span>
              <h3>{item.name}</h3>
              <p>{item.description || item.error}</p>
              <code>
                {item.cases} cases · {item.id}
              </code>
              <div className="button-row">
                <button
                  className="primary"
                  disabled={!item.valid || timelineRun.isPending || suiteRun.isPending}
                  onClick={() => {
                    setMessage("");
                    setReport(undefined);
                    suiteRun.mutate(item.id);
                  }}
                  type="button"
                >
                  {busyID === item.id ? "Running…" : "Run suite"}
                </button>
              </div>
            </article>
          ))}
        </div>
      </Section>
      {mutationError && <Notice tone="error">{errorMessage(mutationError)}</Notice>}
      {message && <Notice tone={report?.passed ? "success" : "error"}>{message}</Notice>}
      {report && (
        <Section title="Latest result" hint={"suiteId" in report ? report.suiteId : report.scenarioId}>
          <Code value={report} />
        </Section>
      )}
    </div>
  );
}
