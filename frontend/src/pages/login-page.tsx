import { Link, useNavigate } from "react-router-dom";

import { AuthForm } from "../components/auth-form";
import { useAuth } from "../lib/use-auth";

export function LoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();

  return (
    <section className="min-h-screen flex flex-col md:flex-row items-center justify-center gap-8 p-8">
      <div className="max-w-sm space-y-3">
        <p className="text-sm font-medium text-muted-foreground uppercase tracking-wide">Access</p>
        <h2>Sign in to your storage workspace.</h2>
        <p>
          Your session uses local storage for now so development stays simple.
          Secure cookie transport is tracked as a follow-up.
        </p>
      </div>

      <div className="max-w-sm w-full">
        <AuthForm
          onSubmit={async (credentials) => {
            await auth.login(credentials);
            await navigate("/app");
          }}
          submitLabel="Log in"
          subtitle="Use your email and password to access your files."
          title="Welcome back"
        />
        <p className="mt-4 text-sm text-center text-muted-foreground">
          Need an account? <Link to="/signup">Create one</Link>
        </p>
      </div>
    </section>
  );
}
