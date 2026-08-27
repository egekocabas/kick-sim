export type APIResponse = { status: number; data: unknown };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function failureMessage(data: unknown, fallback: string): string {
  if (typeof data === "string" && data.trim()) return data;
  if (!isRecord(data)) return fallback;
  const errors = Array.isArray(data.errors)
    ? data.errors
        .map((item) => (isRecord(item) && typeof item.message === "string" ? item.message : ""))
        .filter(Boolean)
    : [];
  if (errors.length > 0) return errors.join("; ");
  if (typeof data.detail === "string" && data.detail) return data.detail;
  if (typeof data.title === "string" && data.title) return data.title;
  return fallback;
}

export function successful<T>(response: APIResponse): T {
  if (response.status < 200 || response.status >= 300) {
    throw new Error(failureMessage(response.data, `Request failed with HTTP ${response.status}`));
  }
  return response.data as T;
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
