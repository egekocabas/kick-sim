import type { Bootstrap } from "../../api/generated/models";
import { ActivityTable, Section, Stat } from "../../components/ui";
import type { Page } from "../../App";

export function Dashboard({ bootstrap, onNavigate }: { bootstrap: Bootstrap | undefined; onNavigate: (page: Page) => void }) {
  return (
    <div className="page-content">
      <section className="hero-card">
        <p className="eyebrow">Ready on loopback</p>
        <h2>Build, sign, send, and inspect webhooks locally.</h2>
        <p>The browser never receives your private key. Go creates the exact bytes, headers, and signature used for delivery.</p>
        <div className="button-row">
          <button className="primary" onClick={() => onNavigate("Events")} type="button">Build an event</button>
          <button onClick={() => onNavigate("Activity")} type="button">Inspect activity</button>
        </div>
      </section>
      <div className="stat-grid">
        <Stat label="Product" value={bootstrap?.productVersion ?? "…"} />
        <Stat label="API" value={bootstrap ? `v${bootstrap.apiVersion}` : "…"} />
        <Stat label="History" value={bootstrap?.workspace.historyEnabled ? "Enabled" : "Disabled"} />
        <Stat label="Signing" value={bootstrap?.key.matchingPrivateKey ? "Key matched" : "Check key"} />
      </div>
      <Section title="Recent activity" hint="Persistent delivery attempts">
        <ActivityTable items={bootstrap?.recentActivity ?? []} />
      </Section>
    </div>
  );
}
