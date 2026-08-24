import { defineConfig } from "orval";

export default defineConfig({
  studio: {
    input: {
      target: "./openapi.json",
    },
    output: {
      mode: "split",
      target: "./src/api/generated/client.ts",
      schemas: "./src/api/generated/models",
      client: "react-query",
      httpClient: "fetch",
      clean: true,
      override: {
        query: {
          signal: true,
        },
      },
    },
  },
});
