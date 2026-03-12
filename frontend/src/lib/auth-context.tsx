import { createContext, useState } from "react";
import type { ReactNode } from "react";

import { useQuery } from "@tanstack/react-query";

import {
  clearSession,
  login as loginRequest,
  readSession,
  restoreSession,
  signup as signupRequest,
  type SessionState,
} from "./session";

type Credentials = {
  email: string;
  password: string;
};

type AuthContextValue = {
  isLoading: boolean;
  session: SessionState | null;
  login: (credentials: Credentials) => Promise<void>;
  signup: (credentials: Credentials) => Promise<void>;
  logout: () => void;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionState | null>(() => readSession());

  const { isLoading } = useQuery({
    queryKey: ["session-restore"],
    queryFn: async () => {
      const existing = readSession();
      if (!existing) return null;
      try {
        const restored = await restoreSession();
        setSession(restored);
        return restored;
      } catch {
        clearSession();
        setSession(null);
        return null;
      }
    },
    staleTime: Infinity,
    gcTime: 0,
    retry: false,
  });

  const value: AuthContextValue = {
    isLoading,
    session,
    async login(credentials) {
      const nextSession = await loginRequest(credentials);
      setSession(nextSession);
    },
    async signup(credentials) {
      const nextSession = await signupRequest(credentials);
      setSession(nextSession);
    },
    logout() {
      clearSession();
      setSession(null);
    },
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export { AuthContext };
