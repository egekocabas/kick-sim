import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getBootstrap } from "./api/generated/client";
import type { Bootstrap } from "./api/generated/models";
import { Notice } from "./components/ui";
import { ActivityPage } from "./features/activity/ActivityPage";
import { Dashboard } from "./features/dashboard/Dashboard";
import { EventBuilder } from "./features/events/EventBuilder";
import { KeysPage } from "./features/keys/KeysPage";
import { ScenariosPage } from "./features/scenarios/ScenariosPage";
import { APIDocsPage, DestinationsPage, SettingsPage } from "./features/static/StaticPages";
import { WorkflowsPage } from "./features/workflows/WorkflowsPage";
import { successful } from "./lib/api";
import { queryKeys } from "./lib/queryKeys";

export type Page = "Dashboard" | "Events" | "Scenarios" | "Workflows" | "Activity" | "Destinations" | "Keys & setup" | "Settings" | "API docs";

const navigation: Page[] = ["Dashboard", "Events", "Scenarios", "Workflows", "Activity", "Destinations", "Keys & setup", "Settings", "API docs"];

export function App() {
  const [page, setPage] = useState<Page>("Dashboard");
  const [scenarioID, setScenarioID] = useState<string>();
  const bootstrap = useQuery({ queryKey: queryKeys.bootstrap, queryFn: async () => successful<Bootstrap>(await getBootstrap()) });
  return (
    <div className="studio-shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark">K</span><span><strong>Kick Sim</strong><small>Local Studio</small></span></div>
        <nav aria-label="Studio navigation">
          {navigation.map((item) => (
            <button className={item === page ? "nav-item active" : "nav-item"} key={item} onClick={() => setPage(item)} type="button">
              <span className="nav-dot" />{item}
            </button>
          ))}
        </nav>
        <div className="sidebar-foot"><span className="status-dot" /><span><strong>Local only</strong><small>127.0.0.1</small></span></div>
      </aside>
      <main>
        <header className="topbar">
          <div><p className="eyebrow">Kick webhook workbench</p><h1>{page}</h1></div>
          <div className="workspace-chip"><span className="status-dot" /><span><small>Workspace</small><strong>{bootstrap.data?.workspace.path ?? "Loading…"}</strong></span></div>
        </header>
        {bootstrap.isError && <Notice tone="error">{bootstrap.error.message}</Notice>}
        {page === "Dashboard" && <Dashboard bootstrap={bootstrap.data} onNavigate={setPage} />}
        {page === "Events" && <EventBuilder initialScenarioID={scenarioID} />}
        {page === "Scenarios" && <ScenariosPage onOpen={(id) => { setScenarioID(id); setPage("Events"); }} />}
        {page === "Workflows" && <WorkflowsPage />}
        {page === "Activity" && <ActivityPage />}
        {page === "Destinations" && <DestinationsPage bootstrap={bootstrap.data} />}
        {page === "Keys & setup" && <KeysPage />}
        {page === "Settings" && <SettingsPage bootstrap={bootstrap.data} />}
        {page === "API docs" && <APIDocsPage />}
      </main>
    </div>
  );
}
