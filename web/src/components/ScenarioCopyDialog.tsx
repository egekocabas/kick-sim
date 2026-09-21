import { useState } from "react";
import { errorMessage } from "../lib/api";
import { Dialog } from "./Dialog";
import { Notice } from "./ui";

export type ScenarioCopyValues = { targetId: string; name: string };

export function ScenarioCopyDialog({
  source,
  onSave,
  onClose,
}: {
  source: { id: string; name: string };
  onSave: (values: ScenarioCopyValues) => Promise<unknown>;
  onClose: () => void;
}) {
  const [targetId, setTargetId] = useState(`${source.id.replace(/^builtin:/, "")}-copy`);
  const [name, setName] = useState(`${source.name || source.id} (copy)`);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const validID = /^[a-z0-9][a-z0-9_-]*(\/[a-z0-9][a-z0-9_-]*)*$/.test(targetId.trim());
  return (
    <Dialog title="Save scenario copy" busy={busy} onClose={onClose}>
      <form
        onSubmit={async (event) => {
          event.preventDefault();
          if (busy || !validID || !name.trim()) return;
          setBusy(true);
          setError("");
          try {
            await onSave({ targetId: targetId.trim(), name: name.trim() });
            onClose();
          } catch (problem) {
            setError(errorMessage(problem));
            setBusy(false);
          }
        }}
      >
        <p>Create a custom scenario you can edit and run from your library.</p>
        <label>
          Scenario name
          <input required value={name} onChange={(event) => setName(event.target.value)} disabled={busy} />
        </label>
        <label>
          Scenario ID
          <input
            required
            value={targetId}
            onChange={(event) => setTargetId(event.target.value)}
            aria-invalid={!validID}
            aria-describedby="scenario-id-hint"
            disabled={busy}
          />
        </label>
        <p id="scenario-id-hint" className="dialog-hint">
          Use lowercase letters, numbers, hyphens or underscores, separated by slashes. The builtin: prefix is reserved.
        </p>
        {error && <Notice tone="error">{error}</Notice>}
        <div className="button-row dialog-actions">
          <button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button className="primary" type="submit" disabled={busy || !validID || !name.trim()}>
            {busy ? "Saving…" : "Save copy"}
          </button>
        </div>
      </form>
    </Dialog>
  );
}
