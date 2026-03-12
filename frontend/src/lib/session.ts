import {
  getApiV1Me,
  postApiV1AuthLogin,
  postApiV1AuthRefresh,
  postApiV1AuthSignup,
} from "./api/generated";
import type {
  GithubComAbhishekPenDriveBackendInternalApiDtoAuthenticatedUser,
  GithubComAbhishekPenDriveBackendInternalApiDtoCredentialsRequest,
} from "./api/generated";
import { apiClient } from "./api/http";

type SessionState = {
  accessToken: string;
  refreshToken: string;
  user: GithubComAbhishekPenDriveBackendInternalApiDtoAuthenticatedUser;
};

type AuthPayload = {
  tokens?: {
    access_token?: string;
    refresh_token?: string;
  };
  user?: GithubComAbhishekPenDriveBackendInternalApiDtoAuthenticatedUser;
};

const sessionStorageKey = "pen-drive.session";

// TODO: move refresh token transport to secure HTTP-only cookies.
export function readSession(): SessionState | null {
  const raw = window.localStorage.getItem(sessionStorageKey);
  if (!raw) {
    return null;
  }

  try {
    return JSON.parse(raw) as SessionState;
  } catch {
    window.localStorage.removeItem(sessionStorageKey);
    return null;
  }
}

export function writeSession(session: SessionState) {
  window.localStorage.setItem(sessionStorageKey, JSON.stringify(session));
}

export function clearSession() {
  window.localStorage.removeItem(sessionStorageKey);
}

export async function signup(
  credentials: GithubComAbhishekPenDriveBackendInternalApiDtoCredentialsRequest,
): Promise<SessionState> {
  const { data, error } = await postApiV1AuthSignup({
    client: apiClient,
    body: credentials,
  });
  if (error) {
    throw new Error(error.error?.message ?? "signup failed");
  }

  const session = toSessionState(data);
  writeSession(session);
  return session;
}

export async function login(
  credentials: GithubComAbhishekPenDriveBackendInternalApiDtoCredentialsRequest,
): Promise<SessionState> {
  const { data, error } = await postApiV1AuthLogin({
    client: apiClient,
    body: credentials,
  });
  if (error) {
    throw new Error(error.error?.message ?? "login failed");
  }

  const session = toSessionState(data);
  writeSession(session);
  return session;
}

export async function refreshSession(
  currentRefreshToken: string,
): Promise<SessionState> {
  const { data: refreshData, error: refreshError } = await postApiV1AuthRefresh({
    client: apiClient,
    body: { refresh_token: currentRefreshToken },
  });

  if (refreshError) {
    clearSession();
    throw new Error(refreshError.error?.message ?? "refresh failed");
  }

  if (!refreshData.tokens?.access_token) {
    clearSession();
    throw new Error("refresh token response is incomplete");
  }

  const { data: userData, error: userError } = await getApiV1Me({
    client: apiClient,
    headers: {
      Authorization: `Bearer ${refreshData.tokens.access_token}`,
    },
  });

  if (userError) {
    clearSession();
    throw new Error(userError.error?.message ?? "session bootstrap failed");
  }

  const session = toSessionState({
    tokens: refreshData.tokens,
    user: userData,
  });
  writeSession(session);
  return session;
}

export async function restoreSession(): Promise<SessionState | null> {
  const current = readSession();
  if (!current) {
    return null;
  }

  return refreshSession(current.refreshToken);
}

function toSessionState(response: AuthPayload): SessionState {
  if (
    !response.user ||
    !response.tokens?.access_token ||
    !response.tokens.refresh_token
  ) {
    throw new Error("response payload is incomplete");
  }

  return {
    accessToken: response.tokens.access_token,
    refreshToken: response.tokens.refresh_token,
    user: response.user,
  };
}

export type { SessionState };
