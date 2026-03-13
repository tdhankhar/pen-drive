import React from "react";
import { Navigate, useSearchParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { Badge } from "@/components/ui/badge";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { UploadPanel } from "../components/upload-panel";
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
        onUploaded={() =>
          queryClient.invalidateQueries({
            queryKey: getApiV1FilesQueryKey({ client: apiClient }),
          })
        }
      />
      <section className="rounded-lg border bg-card">
        {isLoading ? (
          <p className="p-4 text-sm text-muted-foreground">
            Loading folder contents...
          </p>
        ) : null}
        {error ? <p className="p-4 text-sm text-destructive">{(error as Error).message}</p> : null}
        {!isLoading && !error ? (
          <ul className="divide-y">
            {listing?.entries?.length ? (
              listing.entries.map((entry) => (
                <li key={`${entry.type}:${entry.path}`}>
                  <button
                    className="w-full flex items-center gap-3 px-4 py-3 hover:bg-muted/50 text-left disabled:opacity-50 disabled:cursor-default"
                    disabled={entry.type !== "folder"}
                    onClick={() => {
                      if (entry.type === "folder" && entry.path) {
                        openPath(entry.path);
                      }
                    }}
                    type="button"
                  >
                    {entry.type === "folder" ? (
                      <Badge variant="secondary" className="shrink-0">
                        DIR
                      </Badge>
                    ) : (
                      <Badge variant="outline" className="shrink-0">
                        FILE
                      </Badge>
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
                </li>
              ))
            ) : (
              <li className="p-4 text-sm text-muted-foreground">
                This folder is empty.
              </li>
            )}
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
