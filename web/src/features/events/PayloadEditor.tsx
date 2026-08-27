import { useEffect, useState } from "react";
import { friendly, Notice } from "../../components/ui";

export type JSONObject = Record<string, unknown>;

export function ObjectEditor({ value, onChange }: { value: JSONObject; onChange: (value: JSONObject) => void }) {
  return (
    <div className="form-content">
      {Object.entries(value).map(([key, item]) => (
        <ValueEditor key={key} label={key} value={item} onChange={(next) => onChange({ ...value, [key]: next })} />
      ))}
    </div>
  );
}

function ValueEditor({
  label,
  value,
  onChange,
}: {
  label: string;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  if (value !== null && !Array.isArray(value) && typeof value === "object") {
    return (
      <fieldset>
        <legend>{friendly(label)}</legend>
        <div className="nested-fields">
          {Object.entries(value as JSONObject).map(([key, item]) => (
            <ValueEditor
              key={key}
              label={key}
              value={item}
              onChange={(next) => onChange({ ...(value as JSONObject), [key]: next })}
            />
          ))}
        </div>
      </fieldset>
    );
  }
  if (typeof value === "boolean") {
    return (
      <label className="check-field">
        <input type="checkbox" checked={value} onChange={(event) => onChange(event.target.checked)} />
        {friendly(label)}
      </label>
    );
  }
  if (Array.isArray(value) || value === null) {
    return <StructuredValueEditor label={label} value={value} onChange={onChange} />;
  }
  return (
    <label>
      {friendly(label)}
      <input
        value={String(value)}
        type={typeof value === "number" ? "number" : "text"}
        onChange={(event) => onChange(typeof value === "number" ? Number(event.target.value) : event.target.value)}
      />
    </label>
  );
}

function StructuredValueEditor({
  label,
  value,
  onChange,
}: {
  label: string;
  value: unknown[] | null;
  onChange: (value: unknown) => void;
}) {
  const canonical = JSON.stringify(value, null, 2);
  const [text, setText] = useState(canonical);
  const [error, setError] = useState("");
  useEffect(() => {
    setText(canonical);
    setError("");
  }, [canonical]);
  return (
    <label>
      {friendly(label)}
      <textarea
        rows={Math.min(8, Math.max(2, text.split("\n").length))}
        value={text}
        onChange={(event) => {
          const next = event.target.value;
          setText(next);
          try {
            onChange(JSON.parse(next));
            setError("");
          } catch (problem) {
            setError(problem instanceof Error ? problem.message : String(problem));
          }
        }}
      />
      {error && <Notice tone="error">{error}</Notice>}
    </label>
  );
}
