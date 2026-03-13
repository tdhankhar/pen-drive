import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatBytes(bytes: number) {
  const megabytes = bytes / (1024 * 1024)
  return `${megabytes.toFixed(megabytes >= 10 ? 0 : 1)} MB`
}
