import { createClient } from './generated/client';
import { readSession } from '../session';

export const apiClient = createClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? 'http://127.0.0.1:8080',
});

apiClient.interceptors.request.use((request) => {
  const session = readSession();
  if (session?.accessToken) {
    request.headers.set('Authorization', `Bearer ${session.accessToken}`);
  }
  return request;
});
