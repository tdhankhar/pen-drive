export const apiBaseUrl =
  import.meta.env.VITE_API_BASE_URL ?? "http://127.0.0.1:8080";

export async function customFetch<T>(
  url: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  if (!(options.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(`${apiBaseUrl}${url}`, {
    ...options,
    headers,
  });

  const data =
    response.status === 204 ? undefined : ((await response.json()) as T);

  return {
    data: data as T,
    headers: response.headers,
    status: response.status,
  } as T;
}
