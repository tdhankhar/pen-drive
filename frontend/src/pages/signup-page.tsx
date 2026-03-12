import { Link, useNavigate } from "react-router-dom";

import { AuthForm } from "../components/auth-form";
import { useAuth } from "../lib/use-auth";

export function SignupPage() {
  const auth = useAuth();
  const navigate = useNavigate();

  return (
    <section className="min-h-screen flex flex-col md:flex-row items-center justify-center gap-8 p-8">
      <div className="max-w-sm space-y-3">
        <p className="text-sm font-medium text-muted-foreground uppercase tracking-wide">Provisioning</p>
        <h2>Create your account and bucket in one step.</h2>
        <p>
          Signup provisions a per-user storage bucket through the backend and
          drops you into the dashboard shell immediately after success.
        </p>
      </div>

      <div className="max-w-sm w-full">
        <AuthForm
          onSubmit={async (credentials) => {
            await auth.signup(credentials);
            await navigate("/app");
          }}
          submitLabel="Create account"
          subtitle="Use a real email shape and an 8+ character password."
          title="Start a workspace"
        />
        <p className="mt-4 text-sm text-center text-muted-foreground">
          Already registered? <Link to="/login">Log in</Link>
        </p>
      </div>
    </section>
  );
}
