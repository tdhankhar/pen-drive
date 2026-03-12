import { Navigate, Outlet, Route, Routes } from "react-router-dom";

import { useAuth } from "./lib/use-auth";
import { DashboardPage } from "./pages/dashboard-page";
import { LoginPage } from "./pages/login-page";
import { SignupPage } from "./pages/signup-page";

function ProtectedLayout() {
  const auth = useAuth();

  if (auth.isLoading) {
    return <div className="screen-state">Restoring session...</div>;
  }

  if (!auth.session) {
    return <Navigate replace to="/login" />;
  }

  return <Outlet />;
}

function PublicLayout() {
  const auth = useAuth();

  if (auth.isLoading) {
    return <div className="screen-state">Restoring session...</div>;
  }

  if (auth.session) {
    return <Navigate replace to="/app" />;
  }

  return <Outlet />;
}

export function AppRouter() {
  return (
    <Routes>
      <Route element={<PublicLayout />}>
        <Route element={<LoginPage />} path="/login" />
        <Route element={<SignupPage />} path="/signup" />
      </Route>

      <Route element={<ProtectedLayout />}>
        <Route element={<DashboardPage />} path="/app" />
      </Route>

      <Route element={<Navigate replace to="/app" />} path="*" />
    </Routes>
  );
}
