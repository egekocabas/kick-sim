import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ScenarioCopyDialog } from "./ScenarioCopyDialog";

beforeEach(() => {
  HTMLDialogElement.prototype.showModal = function () {
    this.setAttribute("open", "");
  };
  HTMLDialogElement.prototype.close = function () {
    this.removeAttribute("open");
  };
});
afterEach(cleanup);

it("keeps the edited name and ID after a failed save and allows retry", async () => {
  const onSave = vi.fn().mockRejectedValueOnce(new Error("Scenario already exists")).mockResolvedValueOnce({});
  const onClose = vi.fn();
  render(
    <ScenarioCopyDialog
      source={{ id: "builtin:chat/basic-message", name: "Basic message" }}
      onSave={onSave}
      onClose={onClose}
    />,
  );
  fireEvent.change(screen.getByLabelText("Scenario name"), { target: { value: " My chat " } });
  fireEvent.click(screen.getByRole("button", { name: "Save copy" }));
  await screen.findByText("Scenario already exists");
  expect(onClose).not.toHaveBeenCalled();
  expect((screen.getByLabelText("Scenario name") as HTMLInputElement).value).toBe(" My chat ");
  fireEvent.change(screen.getByLabelText("Scenario ID"), { target: { value: "custom/my-chat" } });
  fireEvent.click(screen.getByRole("button", { name: "Save copy" }));
  await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
  expect(onSave).toHaveBeenLastCalledWith({ targetId: "custom/my-chat", name: "My chat" });
});

it("cancels without saving and rejects reserved IDs", () => {
  const onSave = vi.fn();
  const onClose = vi.fn();
  render(
    <ScenarioCopyDialog source={{ id: "livestream/custom", name: "My stream" }} onSave={onSave} onClose={onClose} />,
  );
  fireEvent.change(screen.getByLabelText("Scenario ID"), { target: { value: "builtin:livestream/custom-copy" } });
  expect((screen.getByRole("button", { name: "Save copy" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
  expect(onClose).toHaveBeenCalledOnce();
  expect(onSave).not.toHaveBeenCalled();
});
