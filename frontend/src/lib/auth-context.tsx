import {
  createContext,
  startTransition,
  useEffect,
  useRef,
  useState,
} from "react";
import type { ReactNode } from "react";

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
  const [isLoading, setIsLoading] = useState(true);
  const hasBootstrapped = useRef(false);

  useEffect(() => {
    if (hasBootstrapped.current) {
      return;
    }

    hasBootstrapped.current = true;
    let cancelled = false;

    const bootstrap = async () => {
      const existing = readSession();
      if (!existing) {
        setIsLoading(false);
        return;
      }

      try {
        const restored = await restoreSession();
        if (!cancelled) {
          startTransition(() => {
            setSession(restored);
            setIsLoading(false);
          });
        }
      } catch {
        if (!cancelled) {
          clearSession();
          startTransition(() => {
            setSession(null);
            setIsLoading(false);
          });
        }
      }
    };

    void bootstrap();

    return () => {
      cancelled = true;
    };
  }, []);

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
