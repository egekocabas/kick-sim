export const queryKeys = {
  bootstrap: ["bootstrap"] as const,
  scenarios: ["scenarios"] as const,
  validScenarios: ["scenarios", "valid"] as const,
  timelineScenarios: ["scenarios", "timelines"] as const,
  scenario: (id: string) => ["scenario", id] as const,
  scenarioSource: (id: string) => ["scenario-source", id] as const,
  suites: ["suites"] as const,
  activity: ["activity"] as const,
  attempt: (id: string) => ["attempt", id] as const,
  key: ["key"] as const,
  publicKey: ["public-key"] as const,
  preview: (input: unknown) => ["event-preview", input] as const,
};
