import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";

const scenario = {
  builtIn: true,
  description: "A basic chat message",
  destination: "local",
  draftPayload: { content: "Hello", sender: { username: "viewer", user_id: 42, is_verified: false } },
  eventType: "chat.message.sent",
  eventVersion: 1,
  expectedStatuses: [200],
  id: "builtin:chat/basic-message",
  name: "Basic message",
  payload: {},
  revision: "abc",
  source: "version: 1\nname: Basic message\n",
  sourceFormat: "yaml",
  sourceVersion: 1,
  valid: true,
};

function response(data: unknown, status = 200) {
  return Promise.resolve(new Response(JSON.stringify(data), { status, headers: { "Content-Type": "application/json" } }));
}

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
    const url = String(input);
    if (url === "/api/bootstrap") return response({ apiVersion: 1, capabilities: [], productVersion: "0.2.0", recentActivity: [], workspace: { path: "/tmp/.kick-sim", defaultDestination: "local", destinationUrl: "http://127.0.0.1:3000/webhooks/kick", historyEnabled: true, databasePath: "/tmp/.kick-sim/.runtime/kick-sim.db" }, key: { algorithm: "RSA", bits: 2048, fingerprint: "test", matchingPrivateKey: true, modifiedAt: new Date().toISOString(), path: "/tmp/public.pem" } });
    if (url === "/api/scenarios") return response({ items: [scenario] });
    if (url.startsWith("/api/scenario?")) return response(scenario);
    if (url === "/api/events/validate") return response({ valid: true });
    throw new Error(`Unexpected request ${url}`);
  }));
});

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe("event editor", () => {
  it("keeps form and raw JSON on one semantic draft and blocks invalid JSON", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><App /></QueryClientProvider>);
    fireEvent.click(await screen.findByRole("button", { name: "Build an event" }));
    const content = await screen.findByLabelText("Content");
    fireEvent.change(content, { target: { value: "Changed in form" } });
    fireEvent.click(screen.getByRole("tab", { name: "Raw JSON Payload" }));
    const raw = screen.getByLabelText("Raw JSON payload") as HTMLTextAreaElement;
    expect(raw.value).toContain("Changed in form");
    fireEvent.change(raw, { target: { value: "{" } });
    await waitFor(() => expect(screen.getByText(/JSON at position/)).toBeTruthy());
    expect((screen.getByRole("button", { name: /Send event/ }) as HTMLButtonElement).disabled).toBe(true);
  });
});

describe("scenario source editor", () => {
  const custom = { ...scenario, builtIn: false, id: "editing/basic", revision: `sha256:${"a".repeat(64)}`, source: "version: 1\nname: Basic message\n" };

  it("saves the exact source draft with its loaded revision", async () => {
    let submitted: Record<string, string> | undefined;
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "/api/bootstrap") return response({ apiVersion: 1, capabilities: [], productVersion: "dev", recentActivity: [], workspace: { path: "/tmp/.kick-sim", defaultDestination: "local", destinationUrl: "http://127.0.0.1:3000/webhooks/kick", historyEnabled: true, databasePath: "/tmp/.kick-sim/.runtime/kick-sim.db" }, key: { algorithm: "RSA", bits: 2048, fingerprint: "test", matchingPrivateKey: true, modifiedAt: new Date().toISOString(), path: "/tmp/public.pem" } });
      if (url === "/api/scenarios") return response({ items: [custom] });
      if (url.startsWith("/api/scenario?") && (!init?.method || init.method === "GET")) return response(custom);
      if (url === "/api/scenario" && init?.method === "PUT") {
        submitted = JSON.parse(String(init.body));
        return response({ ...custom, revision: `sha256:${"b".repeat(64)}`, source: submitted?.source });
      }
      throw new Error(`Unexpected request ${url}`);
    }));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><App /></QueryClientProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Scenarios" }));
    fireEvent.click(await screen.findByRole("button", { name: "Edit source" }));
    const editor = await screen.findByLabelText("Scenario source") as HTMLTextAreaElement;
    const changed = `${editor.value}description: Kept exactly\n`;
    fireEvent.change(editor, { target: { value: changed } });
    fireEvent.click(screen.getByRole("button", { name: "Save source" }));
    await screen.findByText("Source saved");
    expect(submitted).toEqual({ id: custom.id, revision: custom.revision, source: changed });
  });

  it("warns about an external revision without replacing the open draft", async () => {
    let current = custom;
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/bootstrap") return response({ apiVersion: 1, capabilities: [], productVersion: "dev", recentActivity: [], workspace: { path: "/tmp/.kick-sim", defaultDestination: "local", destinationUrl: "http://127.0.0.1:3000/webhooks/kick", historyEnabled: true, databasePath: "/tmp/.kick-sim/.runtime/kick-sim.db" }, key: { algorithm: "RSA", bits: 2048, fingerprint: "test", matchingPrivateKey: true, modifiedAt: new Date().toISOString(), path: "/tmp/public.pem" } });
      if (url === "/api/scenarios") return response({ items: [current] });
      if (url.startsWith("/api/scenario?")) return response(current);
      throw new Error(`Unexpected request ${url}`);
    }));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><App /></QueryClientProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Scenarios" }));
    fireEvent.click(await screen.findByRole("button", { name: "Edit source" }));
    const editor = await screen.findByLabelText("Scenario source") as HTMLTextAreaElement;
    const openDraft = `${editor.value}description: Unsaved Studio draft\n`;
    fireEvent.change(editor, { target: { value: openDraft } });
    current = { ...custom, revision: `sha256:${"c".repeat(64)}`, source: "version: 1\nname: External edit\n" };
    await act(async () => { await client.invalidateQueries({ queryKey: ["scenario-source", custom.id] }); });
    await screen.findByText("File changed on disk");
    expect(editor.value).toBe(openDraft);
    fireEvent.click(screen.getByRole("button", { name: "Keep draft" }));
    expect(editor.value).toBe(openDraft);
  });

  it("keeps invalid external files visible for source repair", async () => {
    const invalid = { ...custom, name: "", eventType: "", eventVersion: 0, valid: false, source: "version: [\n", validationErrors: ["invalid YAML"] };
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/bootstrap") return response({ apiVersion: 1, capabilities: [], productVersion: "dev", recentActivity: [], workspace: { path: "/tmp/.kick-sim", defaultDestination: "local", destinationUrl: "http://127.0.0.1:3000/webhooks/kick", historyEnabled: true, databasePath: "/tmp/.kick-sim/.runtime/kick-sim.db" }, key: { algorithm: "RSA", bits: 2048, fingerprint: "test", matchingPrivateKey: true, modifiedAt: new Date().toISOString(), path: "/tmp/public.pem" } });
      if (url === "/api/scenarios") return response({ items: [invalid] });
      if (url.startsWith("/api/scenario?")) return response(invalid);
      throw new Error(`Unexpected request ${url}`);
    }));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><App /></QueryClientProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Scenarios" }));
    await screen.findByText("Invalid source");
    expect((screen.getByRole("button", { name: "Open" }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "Edit source" }));
    expect((await screen.findByLabelText("Scenario source") as HTMLTextAreaElement).value).toBe(invalid.source);
    expect(screen.getAllByText("invalid YAML").length).toBeGreaterThan(0);
  });
});
