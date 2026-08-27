import type { Bootstrap } from "../../api/generated/models";
import { Info, Notice, Section } from "../../components/ui";

export function DestinationsPage({ bootstrap }: { bootstrap: Bootstrap | undefined }) {
  return (
    <div className="page-content">
      <Section title="Destination" hint="Delivery is restricted to loopback receivers">
        <div className="detail-grid">
          <Info label="Name" value={bootstrap?.workspace.defaultDestination ?? "…"} />
          <Info label="URL" value={bootstrap?.workspace.destinationUrl ?? "…"} />
        </div>
        <Notice>Temporary URLs can be supplied from the event builder and are never saved.</Notice>
      </Section>
    </div>
  );
}

export function SettingsPage({ bootstrap }: { bootstrap: Bootstrap | undefined }) {
  return (
    <div className="page-content">
      <Section title="Runtime settings" hint="Read-only workspace summary">
        <div className="detail-grid">
          <Info label="Workspace" value={bootstrap?.workspace.path ?? "…"} />
          <Info label="Database" value={bootstrap?.workspace.databasePath ?? "…"} />
          <Info label="History" value={bootstrap?.workspace.historyEnabled ? "Enabled" : "Disabled"} />
          <Info label="Capabilities" value={bootstrap?.capabilities?.join(", ") ?? "…"} />
        </div>
      </Section>
    </div>
  );
}

export function APIDocsPage() {
  return (
    <div className="page-content">
      <Section title="Local API" hint="OpenAPI 3.1, generated from the Go contracts">
        <p>Use the committed document for client generation or inspect the live document exposed by this process.</p>
        <div className="button-row">
          <a className="button primary" href="/api/openapi" target="_blank" rel="noreferrer">
            Open live OpenAPI
          </a>
          <a className="button" href="/api/capabilities" target="_blank" rel="noreferrer">
            View capabilities
          </a>
        </div>
      </Section>
    </div>
  );
}
