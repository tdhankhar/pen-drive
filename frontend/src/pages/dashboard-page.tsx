import React from "react";
import { Navigate, useSearchParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Folder, FileText, ArrowUp, Trash2, Download } from "lucide-react";
import { UploadPanel } from "../components/upload-panel";
import { deleteApiV1Files } from "../lib/api/generated";
import {
  getApiV1FilesOptions,
  getApiV1FilesQueryKey,
} from "../lib/api/generated/@tanstack/react-query.gen";
import type {
  GithubComAbhishekPenDriveBackendInternalApiDtoFileSystemEntry,
} from "../lib/api/generated";
import { apiClient } from "../lib/api/http";
import { useAuth } from "../lib/use-auth";

export function DashboardPage() {
  const {
    state: { session },
    actions: { logout },
  } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const currentPath = searchParams.get("path") ?? "";
  const segments = currentPath.split("/").filter(Boolean);
  const [deleteError, setDeleteError] = React.useState<string | null>(null);
  const [deletingPath, setDeletingPath] = React.useState<string | null>(null);

  const queryClient = useQueryClient();

  const { data: listing, isLoading, error } = useQuery({
    ...getApiV1FilesOptions({
      client: apiClient,
      query: currentPath ? { path: currentPath } : undefined,
    }),
    enabled: !!session,
  });
  if (!session) {
    return <Navigate replace to="/login" />;
  }

  function openPath(path: string) {
    if (path) {
      setSearchParams({ path });
      return;
    }

    setSearchParams({});
  }

  async function refreshListing() {
    await queryClient.invalidateQueries({
      queryKey: getApiV1FilesQueryKey({ client: apiClient }),
    });
  }

  async function handleDelete(entry: GithubComAbhishekPenDriveBackendInternalApiDtoFileSystemEntry) {
    if (!entry.path || !entry.type) {
      return;
    }

    const label = entry.type === "folder" ? "folder" : "file";
    const confirmed = window.confirm(`Move this ${label} to trash?\n\n${entry.path}`);
    if (!confirmed) {
      return;
    }

    setDeleteError(null);
    setDeletingPath(entry.path);

    try {
      const { error, response } = await deleteApiV1Files({
        client: apiClient,
        query: {
          path: entry.path,
          type: entry.type,
        },
      });

      if (error) {
        const message =
          error.error?.message ||
          `delete failed with status ${response.status}`;
        throw new Error(message);
      }

      await refreshListing();
    } catch (error) {
      setDeleteError(error instanceof Error ? error.message : "delete failed");
    } finally {
      setDeletingPath(null);
    }
  }

  return (
    <main className="min-h-screen p-6 space-y-6 max-w-5xl mx-auto">
      <header className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium text-muted-foreground uppercase tracking-wide">
            Dashboard
          </p>
          <h1>{session.user.email}</h1>
          <p className="text-sm text-muted-foreground">
            User bucket name: <code>{session.user.id}</code>
          </p>
        </div>
        <Button variant="outline" onClick={logout} type="button">
          Log out
        </Button>
      </header>

      <section className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Card>
          <CardHeader>
            <p className="text-sm font-medium text-muted-foreground">Current folder</p>
            <CardTitle>{currentPath || "Root"}</CardTitle>
          </CardHeader>
          <CardContent>
            <p>
              Browse the authenticated user's bucket path and step into nested
              folders from here.
            </p>
            <Breadcrumb className="mt-4">
              <BreadcrumbList>
                <BreadcrumbItem>
                  <button
                    className="text-sm hover:text-foreground text-muted-foreground"
                    onClick={() => openPath("")}
                    type="button"
                  >
                    root
                  </button>
                </BreadcrumbItem>
                {segments.map((segment, index) => {
                  const path = segments.slice(0, index + 1).join("/");
                  const isLast = index === segments.length - 1;
                  return (
                    <React.Fragment key={path}>
                      <BreadcrumbSeparator />
                      <BreadcrumbItem>
                        {isLast ? (
                          <BreadcrumbPage>{segment}</BreadcrumbPage>
                        ) : (
                          <button
                            className="text-sm hover:text-foreground text-muted-foreground"
                            onClick={() => openPath(path)}
                            type="button"
                          >
                            {segment}
                          </button>
                        )}
                      </BreadcrumbItem>
                    </React.Fragment>
                  );
                })}
              </BreadcrumbList>
            </Breadcrumb>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <p className="text-sm font-medium text-muted-foreground">Listing status</p>
            <CardTitle>
              {isLoading ? "Loading" : `${listing?.entries?.length ?? 0} entries`}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p>
              Pagination token exposed by backend:{" "}
              <code>{listing?.next_continuation_token || "none"}</code>
            </p>
          </CardContent>
        </Card>
      </section>

      <UploadPanel
        currentPath={currentPath}
        onUploaded={refreshListing}
      />
      <section className="rounded-lg border bg-card">
        {isLoading ? (
          <p className="p-4 text-sm text-muted-foreground">
            Loading folder contents...
          </p>
        ) : null}
        {error ? <p className="p-4 text-sm text-destructive">{(error as Error).message}</p> : null}
        {deleteError ? <p className="p-4 text-sm text-destructive">{deleteError}</p> : null}
        {!isLoading && !error ? (
          <ul className="divide-y">
            {currentPath ? (
              <li key="go-up">
                <button
                  className="w-full flex items-center gap-3 px-4 py-3 hover:bg-muted/50 text-left"
                  onClick={() => openPath(segments.slice(0, -1).join("/"))}
                  type="button"
                >
                  <ArrowUp className="w-5 h-5 text-muted-foreground shrink-0" />
                  <span className="font-medium text-sm flex-1 truncate">
                    ..
                  </span>
                  <span className="text-xs text-muted-foreground shrink-0">
                    Go up
                  </span>
                </button>
              </li>
            ) : null}
            {listing?.entries?.map((entry) => (
              <li key={`${entry.type}:${entry.path}`}>
                <div className="flex items-center gap-3 px-4 py-3 hover:bg-muted/50">
                  <button
                    className="flex min-w-0 flex-1 items-center gap-3 text-left disabled:cursor-default disabled:opacity-50"
                    disabled={entry.type !== "folder"}
                    onClick={() => {
                      if (entry.type === "folder" && entry.path) {
                        openPath(entry.path);
                      }
                    }}
                    type="button"
                  >
                    {entry.type === "folder" ? (
                      <Folder className="w-5 h-5 text-muted-foreground shrink-0" />
                    ) : (
                      <FileText className="w-5 h-5 text-muted-foreground shrink-0" />
                    )}
                    <span className="font-medium text-sm flex-1 truncate">
                      {entry.name}
                    </span>
                    <span className="text-xs text-muted-foreground truncate max-w-xs">
                      {entry.path}
                    </span>
                    <span className="text-xs text-muted-foreground shrink-0">
                      {formatEntryMeta(entry)}
                    </span>
                  </button>
                  {entry.type === "file" && entry.presigned_url ? (
                    <Button
                      aria-label={`Download ${entry.path || entry.name || "file"}`}
                      asChild
                      size="icon"
                      type="button"
                      variant="ghost"
                    >
                      <a
                        href={entry.presigned_url}
                        download={entry.name || "download"}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        <Download />
                      </a>
                    </Button>
                  ) : null}
                  <Button
                    aria-label={`Delete ${entry.path || entry.name || "entry"}`}
                    disabled={!entry.path || deletingPath === entry.path}
                    onClick={() => void handleDelete(entry)}
                    size="icon"
                    type="button"
                    variant="ghost"
                  >
                    <Trash2 />
                  </Button>
                  </div>
              </li>
            ))}
            {!listing?.entries?.length ? (
              <li className="p-4 text-sm text-muted-foreground">
                This folder is empty.
              </li>
            ) : null}
          </ul>
        ) : null}
      </section>
    </main>
  );
}

function formatEntryMeta(
  entry: GithubComAbhishekPenDriveBackendInternalApiDtoFileSystemEntry,
) {
  if (entry.type === "folder") {
    return "Open folder";
  }

  const parts = [];
  if (typeof entry.size === "number") {
    parts.push(`${entry.size} bytes`);
  }
  if (entry.last_modified) {
    parts.push(new Date(entry.last_modified).toLocaleString());
  }

  return parts.join(" • ") || "File";
}
