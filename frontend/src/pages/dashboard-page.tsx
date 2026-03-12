import { Navigate } from "react-router-dom";

import { useAuth } from "../lib/use-auth";

export function DashboardPage() {
  const auth = useAuth();

  if (!auth.session) {
    return <Navigate replace to="/login" />;
  }

  return (
    <main className="dashboard-shell">
      <header className="dashboard-header">
        <div>
          <p className="eyebrow">Dashboard</p>
          <h1>{auth.session.user.email}</h1>
          <p className="dashboard-meta">
            User bucket name: <code>{auth.session.user.id}</code>
          </p>
        </div>
        <button className="secondary-button" onClick={auth.logout} type="button">
          Log out
        </button>
      </header>

      <section className="dashboard-grid">
        <article className="panel">
          <p className="panel-label">Session</p>
          <h2>Authenticated and refresh-capable.</h2>
          <p>
            Access and refresh tokens are persisted locally for this phase.
            Replace this with secure cookie transport later.
          </p>
        </article>
        <article className="panel">
          <p className="panel-label">Next backend hookup</p>
          <h2>File browser routes plug in here.</h2>
          <p>
            The next frontend checkpoint can connect list/download endpoints into
            this dashboard shell.
          </p>
        </article>
      </section>
    </main>
  );
}
