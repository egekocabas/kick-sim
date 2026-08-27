import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ObjectEditor } from "./PayloadEditor";

afterEach(cleanup);

describe("structured payload values", () => {
  it("keeps invalid text visible and reports the parse error", () => {
    const onChange = vi.fn();
    render(<ObjectEditor value={{ tags: ["first"] }} onChange={onChange} />);
    const editor = screen.getByLabelText("Tags") as HTMLTextAreaElement;

    fireEvent.change(editor, { target: { value: "[" } });

    expect(editor.value).toBe("[");
    expect(screen.getByRole("alert").textContent).toMatch(/JSON/);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("commits a valid array after an invalid edit", () => {
    const onChange = vi.fn();
    render(<ObjectEditor value={{ tags: ["first"] }} onChange={onChange} />);
    const editor = screen.getByLabelText("Tags");
    fireEvent.change(editor, { target: { value: "[" } });
    fireEvent.change(editor, { target: { value: '["second"]' } });

    expect(onChange).toHaveBeenLastCalledWith({ tags: ["second"] });
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
