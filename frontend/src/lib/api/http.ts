import { createClient } from './generated/client';
import { getSessionSnapshot } from '../session';
import { API_BASE_URL } from './base-url';

export const apiClient = createClient({
  baseUrl: API_BASE_URL,
  credentials: 'include',
});

apiClient.interceptors.request.use((request) => {
  const session = getSessionSnapshot();
  if (session?.accessToken) {
    request.headers.set('Authorization', `Bearer ${session.accessToken}`);
  }
  return request;
});
