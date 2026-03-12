import { Link, useNavigate } from "react-router-dom";

import { AuthForm } from "../components/auth-form";
import { useAuth } from "../lib/use-auth";

export function SignupPage() {
  const auth = useAuth();
  const navigate = useNavigate();

  return (
    <section className="auth-layout">
      <div className="auth-panel auth-panel-copy">
        <p className="eyebrow">Provisioning</p>
        <h2>Create your account and bucket in one step.</h2>
        <p>
          Signup provisions a per-user storage bucket through the backend and
          drops you into the dashboard shell immediately after success.
        </p>
      </div>

      <div className="auth-panel">
        <AuthForm
          onSubmit={async (credentials) => {
            await auth.signup(credentials);
            await navigate("/app");
          }}
          submitLabel="Create account"
          subtitle="Use a real email shape and an 8+ character password."
          title="Start a workspace"
        />
        <p className="auth-footer">
          Already registered? <Link to="/login">Log in</Link>
        </p>
      </div>
    </section>
  );
}
