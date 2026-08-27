import type { ScenarioDetail } from "../api/generated/models";

export const scenario: ScenarioDetail = {
  builtIn: true,
  description: "A basic chat message",
  destination: "local",
  draftPayload: {
    content: "Hello",
    sender: { username: "viewer", user_id: 42, is_verified: false },
    tags: ["first"],
  },
  eventType: "chat.message.sent",
  eventVersion: 1,
  expectedStatuses: [200],
  id: "builtin:chat/basic-message",
  kind: "single",
  name: "Basic message",
  payload: {},
  revision: "abc",
  source: "version: 1\nname: Basic message\n",
  sourceFormat: "yaml",
  sourceVersion: 1,
  valid: true,
};

export function jsonResponse(data: unknown, status = 200): Promise<Response> {
  return Promise.resolve(
    new Response(JSON.stringify(data), { status, headers: { "Content-Type": "application/json" } }),
  );
}
