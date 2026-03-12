import { useState } from "react";

import { Button } from "@/components/ui/button";

type AuthFormProps = {
  title: string;
  subtitle: string;
  submitLabel: string;
  onSubmit: (values: { email: string; password: string }) => Promise<void>;
};

export function AuthForm({
  title,
  subtitle,
  submitLabel,
  onSubmit,
}: AuthFormProps) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  return (
    <form
      className="auth-card"
      onSubmit={async (event) => {
        event.preventDefault();
        setError(null);
        setIsSubmitting(true);

        try {
          await onSubmit({ email, password });
        } catch (submitError) {
          setError(
            submitError instanceof Error ? submitError.message : "request failed",
          );
        } finally {
          setIsSubmitting(false);
        }
      }}
    >
      <div className="auth-copy">
        <p className="eyebrow">pen-drive</p>
        <h1>{title}</h1>
        <p>{subtitle}</p>
      </div>

      <label className="field">
        <span>Email</span>
        <input
          autoComplete="email"
          className="w-full rounded-xl border border-border bg-white px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          name="email"
          onChange={(event) => setEmail(event.target.value)}
          placeholder="you@example.com"
          type="email"
          value={email}
        />
      </label>

      <label className="field">
        <span>Password</span>
        <input
          autoComplete="current-password"
          className="w-full rounded-xl border border-border bg-white px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          name="password"
          onChange={(event) => setPassword(event.target.value)}
          placeholder="Minimum 8 characters"
          type="password"
          value={password}
        />
      </label>

      {error ? <p className="form-error">{error}</p> : null}

      <Button type="submit" className="w-full" disabled={isSubmitting}>
        {isSubmitting ? "Working..." : submitLabel}
      </Button>
    </form>
  );
}
