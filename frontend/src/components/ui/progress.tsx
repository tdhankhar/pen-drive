import * as React from "react";

import { cn } from "@/lib/utils";

type ProgressProps = React.ComponentProps<"div"> & {
  value?: number;
};

export function Progress({ className, value = 0, ...props }: ProgressProps) {
  const clampedValue = Math.max(0, Math.min(100, value));

  return (
    <div
      aria-label="Progress"
      aria-valuemax={100}
      aria-valuemin={0}
      aria-valuenow={clampedValue}
      className={cn("relative h-2 w-full overflow-hidden rounded-full bg-muted", className)}
      role="progressbar"
      {...props}
    >
      <div
        className="h-full bg-primary transition-[width] duration-200"
        style={{ width: `${clampedValue}%` }}
      />
    </div>
  );
}
