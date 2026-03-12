import { createClient } from '@hey-api/client-fetch';

export const apiClient = createClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? 'http://127.0.0.1:8080',
});
