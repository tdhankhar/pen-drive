import {
  getApiV1Me,
  postApiV1AuthLogin,
  postApiV1AuthRefresh,
  postApiV1AuthSignup,
} from "./api/generated/client";
import type {
  GithubComAbhishekPenDriveBackendInternalApiDtoAuthenticatedUser,
  GithubComAbhishekPenDriveBackendInternalApiDtoCredentialsRequest,
} from "./api/generated/model";

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
  const response = await postApiV1AuthSignup(credentials);
  if (response.status !== 201) {
    throw new Error(response.data.error?.message ?? "signup failed");
  }

  const session = toSessionState(response.data);
  writeSession(session);
  return session;
}

export async function login(
  credentials: GithubComAbhishekPenDriveBackendInternalApiDtoCredentialsRequest,
): Promise<SessionState> {
  const response = await postApiV1AuthLogin(credentials);
  if (response.status !== 200) {
    throw new Error(response.data.error?.message ?? "login failed");
  }

  const session = toSessionState(response.data);
  writeSession(session);
  return session;
}

export async function refreshSession(
  currentRefreshToken: string,
): Promise<SessionState> {
  const response = await postApiV1AuthRefresh({
    refresh_token: currentRefreshToken,
  });

  if (response.status !== 200) {
    clearSession();
    throw new Error(response.data.error?.message ?? "refresh failed");
  }

  if (!response.data.tokens?.access_token) {
    clearSession();
    throw new Error("refresh token response is incomplete");
  }

  const userResponse = await getApiV1Me({
    headers: {
      Authorization: `Bearer ${response.data.tokens.access_token}`,
    },
  });

  if (userResponse.status !== 200) {
    clearSession();
    throw new Error("session bootstrap failed");
  }

  const session = toSessionState({
    tokens: response.data.tokens,
    user: userResponse.data,
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
