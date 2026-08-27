import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import type { ReactElement } from "react";

export function createTestQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
}

export function renderWithQueryClient(element: ReactElement, client = createTestQueryClient()) {
  return {
    client,
    ...render(<QueryClientProvider client={client}>{element}</QueryClientProvider>),
  };
}
