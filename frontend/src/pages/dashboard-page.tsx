import { useEffect, useState } from "react";
import { Navigate, useSearchParams } from "react-router-dom";

import { getApiV1Files } from "../lib/api/generated/client";
import type {
  GithubComAbhishekPenDriveBackendInternalApiDtoFileListResponse,
  GithubComAbhishekPenDriveBackendInternalApiDtoFileSystemEntry,
} from "../lib/api/generated/model";
import { useAuth } from "../lib/use-auth";

export function DashboardPage() {
  const auth = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const [listing, setListing] =
    useState<GithubComAbhishekPenDriveBackendInternalApiDtoFileListResponse | null>(
      null,
    );
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const session = auth.session;
  const currentPath = searchParams.get("path") ?? "";
  const segments = currentPath.split("/").filter(Boolean);

  useEffect(() => {
    if (!session) {
      return;
    }

    let cancelled = false;

    const load = async () => {
      setIsLoading(true);
      setError(null);

      const response = await getApiV1Files(
        currentPath ? { path: currentPath } : undefined,
        {
          headers: {
            Authorization: `Bearer ${session.accessToken}`,
          },
        },
      );

      if (cancelled) {
        return;
      }

      if (response.status !== 200) {
        setError(response.data.error?.message ?? "listing failed");
        setListing(null);
        setIsLoading(false);
        return;
      }

      setListing(response.data);
      setIsLoading(false);
    };

    void load();

    return () => {
      cancelled = true;
    };
  }, [currentPath, session]);

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
    <main className="dashboard-shell">
      <header className="dashboard-header">
        <div>
          <p className="eyebrow">Dashboard</p>
          <h1>{session.user.email}</h1>
          <p className="dashboard-meta">
            User bucket name: <code>{session.user.id}</code>
          </p>
        </div>
        <button className="secondary-button" onClick={auth.logout} type="button">
          Log out
        </button>
      </header>

      <section className="dashboard-grid">
        <article className="panel">
          <p className="panel-label">Current folder</p>
          <h2>{currentPath || "Root"}</h2>
          <p>
            Browse the authenticated user's bucket path and step into nested
            folders from here.
          </p>
          <nav className="breadcrumb-row" aria-label="Breadcrumb">
            <button
              className="crumb-button"
              onClick={() => openPath("")}
              type="button"
            >
              root
            </button>
            {segments.map((segment, index) => {
              const path = segments.slice(0, index + 1).join("/");
              return (
                <button
                  className="crumb-button"
                  key={path}
                  onClick={() => openPath(path)}
                  type="button"
                >
                  {segment}
                </button>
              );
            })}
          </nav>
        </article>
        <article className="panel">
          <p className="panel-label">Listing status</p>
          <h2>{isLoading ? "Loading" : `${listing?.entries?.length ?? 0} entries`}</h2>
          <p>
            Pagination token exposed by backend:{" "}
            <code>{listing?.next_continuation_token || "none"}</code>
          </p>
        </article>
      </section>

      <section className="browser-panel">
        {isLoading ? <p className="browser-state">Loading folder contents...</p> : null}
        {error ? <p className="browser-state browser-error">{error}</p> : null}
        {!isLoading && !error ? (
          <ul className="entry-list">
            {listing?.entries?.length ? (
              listing.entries.map((entry) => (
                <li className="entry-row" key={`${entry.type}:${entry.path}`}>
                  <button
                    className="entry-button"
                    disabled={entry.type !== "folder"}
                    onClick={() => {
                      if (entry.type === "folder" && entry.path) {
                        openPath(entry.path);
                      }
                    }}
                    type="button"
                  >
                    <span className={`entry-badge entry-${entry.type}`}>
                      {entry.type === "folder" ? "DIR" : "FILE"}
                    </span>
                    <span className="entry-name">{entry.name}</span>
                    <span className="entry-path">{entry.path}</span>
                    <span className="entry-meta">
                      {formatEntryMeta(entry)}
                    </span>
                  </button>
                </li>
              ))
            ) : (
              <li className="browser-state">This folder is empty.</li>
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
