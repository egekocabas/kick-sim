import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
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
  sourceVersion: 1,
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
