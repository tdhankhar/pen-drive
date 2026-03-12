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
      className="grid w-full max-w-[28rem] gap-5 rounded-3xl border border-[rgba(22,138,173,0.16)] bg-white/[0.88] p-8 shadow-[0_20px_50px_rgba(8,43,54,0.08)] backdrop-blur-[14px]"
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
      <div className="grid gap-[0.6rem]">
        <p className="text-sm font-bold uppercase tracking-[0.14em] text-muted-foreground">pen-drive</p>
        <h1>{title}</h1>
        <p>{subtitle}</p>
      </div>

      <label className="flex flex-col gap-[0.45rem] text-sm text-muted-foreground">
        <span className="text-[0.72rem] font-bold uppercase tracking-[0.14em]">Email</span>
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

      <label className="flex flex-col gap-[0.45rem] text-sm text-muted-foreground">
        <span className="text-[0.72rem] font-bold uppercase tracking-[0.14em]">Password</span>
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

      {error ? <p className="rounded-[0.9rem] bg-destructive/[0.08] px-4 py-[0.85rem] text-sm text-[#8f2121]">{error}</p> : null}

      <Button type="submit" className="w-full" disabled={isSubmitting}>
        {isSubmitting ? "Working..." : submitLabel}
      </Button>
    </form>
  );
}
