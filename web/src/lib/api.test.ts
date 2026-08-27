import { describe, expect, it } from "vitest";
import { failureMessage, successful } from "./api";

describe("API response parsing", () => {
  it.each([
    [null, "fallback"],
    [42, "fallback"],
    ["plain failure", "plain failure"],
    [{ title: "Problem title" }, "Problem title"],
    [{ detail: "Problem detail" }, "Problem detail"],
    [{ errors: [{ message: "first" }, null, { message: "second" }] }, "first; second"],
  ])("handles malformed and structured error bodies", (body, expected) => {
    expect(failureMessage(body, "fallback")).toBe(expected);
  });

  it("throws a useful fallback for primitive failed responses", () => {
    expect(() => successful({ status: 500, data: false })).toThrow("Request failed with HTTP 500");
  });
});
