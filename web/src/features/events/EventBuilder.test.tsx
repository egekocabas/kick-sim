import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../lib/queryKeys";
import { jsonResponse, scenario } from "../../test/fixtures";
import { renderWithQueryClient } from "../../test/render";
import { EventBuilder } from "./EventBuilder";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
}

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/scenarios") return jsonResponse({ items: [scenario] });
      if (url.startsWith("/api/scenario?")) return jsonResponse(scenario);
      if (url === "/api/events/validate") return jsonResponse({ valid: true });
      if (url === "/api/events/generate") return jsonResponse({ rawHttp: "POST /webhooks/kick HTTP/1.1" });
      if (url === "/api/events/trigger")
        return jsonResponse({ attemptId: "attempt-1", outcome: "success", status: 200 });
      throw new Error(`Unexpected request ${url}`);
    }),
  );
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("event builder request state", () => {
  it("ignores a stale validation response", async () => {
    const first = deferred<Response>();
    const second = deferred<Response>();
    let validationCalls = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        const url = String(input);
        if (url === "/api/scenarios") return jsonResponse({ items: [scenario] });
        if (url.startsWith("/api/scenario?")) return jsonResponse(scenario);
        if (url === "/api/events/validate") {
          validationCalls++;
          return validationCalls === 1 ? first.promise : second.promise;
        }
        throw new Error(`Unexpected request ${url}`);
      }),
    );
    renderWithQueryClient(<EventBuilder initialScenarioID={undefined} />);
    await screen.findByText("chat.message.sent@1");
    fireEvent.click(await screen.findByRole("tab", { name: "Raw JSON Payload" }));
    const raw = screen.getByLabelText("Raw JSON payload");
    fireEvent.change(raw, { target: { value: JSON.stringify({ content: "first" }) } });
    await waitFor(() => expect(validationCalls).toBe(1));
    fireEvent.change(raw, { target: { value: JSON.stringify({ content: "second" }) } });
    await waitFor(() => expect(validationCalls).toBe(2));

    second.resolve(await jsonResponse({ valid: true }));
    await waitFor(() => expect(screen.queryByText("Validating payload…")).toBeNull());
    first.resolve(await jsonResponse({ detail: "stale failure" }, 422));

    await waitFor(() => expect(screen.queryByText("stale failure")).toBeNull());
    expect((raw as HTMLTextAreaElement).value).toContain("second");
  });

  it("regenerates a signed preview when the destination changes", async () => {
    const previewBodies: unknown[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url === "/api/scenarios") return jsonResponse({ items: [scenario] });
        if (url.startsWith("/api/scenario?")) return jsonResponse(scenario);
        if (url === "/api/events/generate") {
          previewBodies.push(JSON.parse(String(init?.body)));
          return jsonResponse({ rawHttp: `preview-${previewBodies.length}` });
        }
        throw new Error(`Unexpected request ${url}`);
      }),
    );
    renderWithQueryClient(<EventBuilder initialScenarioID={undefined} />);
    await screen.findByText("chat.message.sent@1");
    fireEvent.click(await screen.findByRole("tab", { name: "Raw HTTP Preview" }));
    await screen.findByText("preview-1");
    fireEvent.change(screen.getByPlaceholderText("Use configured destination"), {
      target: { value: "http://127.0.0.1:4000/hooks" },
    });
    await screen.findByText("preview-2");

    expect(previewBodies.at(-1)).toMatchObject({ destinationUrl: "http://127.0.0.1:4000/hooks" });
    expect(screen.getByRole("tabpanel").getAttribute("aria-labelledby")).toBe("editor-tab-http");
  });

  it("shows preview failures instead of an unavailable placeholder", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        const url = String(input);
        if (url === "/api/scenarios") return jsonResponse({ items: [scenario] });
        if (url.startsWith("/api/scenario?")) return jsonResponse(scenario);
        if (url === "/api/events/generate") return jsonResponse({ detail: "preview signing failed" }, 500);
        throw new Error(`Unexpected request ${url}`);
      }),
    );
    renderWithQueryClient(<EventBuilder initialScenarioID={undefined} />);
    await screen.findByText("chat.message.sent@1");
    fireEvent.click(await screen.findByRole("tab", { name: "Raw HTTP Preview" }));
    expect((await screen.findByRole("alert")).textContent).toContain("preview signing failed");
  });

  it("invalidates activity after a successful send", async () => {
    const { client } = renderWithQueryClient(<EventBuilder initialScenarioID={undefined} />);
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const send = (await screen.findByRole("button", { name: /Send event/ })) as HTMLButtonElement;
    await waitFor(() => expect(send.disabled).toBe(false));
    fireEvent.click(send);
    await screen.findByText("success · HTTP 200");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.activity });
  });
});
