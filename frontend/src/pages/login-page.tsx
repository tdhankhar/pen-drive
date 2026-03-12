import { Link, useNavigate } from "react-router-dom";

import { AuthForm } from "../components/auth-form";
import { useAuth } from "../lib/use-auth";

export function LoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();

  return (
    <section className="auth-layout">
      <div className="auth-panel auth-panel-copy">
        <p className="eyebrow">Access</p>
        <h2>Sign in to your storage workspace.</h2>
        <p>
          Your session uses local storage for now so development stays simple.
          Secure cookie transport is tracked as a follow-up.
        </p>
      </div>

      <div className="auth-panel">
        <AuthForm
          onSubmit={async (credentials) => {
            await auth.login(credentials);
            await navigate("/app");
          }}
          submitLabel="Log in"
          subtitle="Use your email and password to access your files."
          title="Welcome back"
        />
        <p className="auth-footer">
          Need an account? <Link to="/signup">Create one</Link>
        </p>
      </div>
    </section>
  );
}
