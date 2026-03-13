import Uppy, { type UploadResult, type UppyFile } from "@uppy/core";
import {
  Dropzone,
  UppyContextProvider,
  useUppyState,
} from "@uppy/react";
import {
  type ChangeEvent,
  type MutableRefObject,
  type ReactNode,
  useEffect,
  useRef,
  useState,
} from "react";

import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Progress } from "@/components/ui/progress";
import { formatBytes } from "@/lib/utils";

import {
  postApiV1FilesDuplicatesPreview,
  postApiV1FilesUploadMultipartAbort,
  postApiV1FilesUploadMultipartComplete,
  postApiV1FilesUploadMultipartInitiate,
  GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy as DuplicateConflictPolicy,
} from "../lib/api/generated";
import type {
  GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy,
  GithubComAbhishekPenDriveBackendInternalApiDtoDuplicatePreviewItem,
  GithubComAbhishekPenDriveBackendInternalApiDtoDuplicatePreviewResponse,
  GithubComAbhishekPenDriveBackendInternalApiDtoFolderUploadResponse,
  GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadPartResponse,
  GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadInitiateResponse,
  PostApiV1FilesUploadData,
} from "../lib/api/generated";
import { apiClient } from "../lib/api/http";
import { API_BASE_URL } from "../lib/api/base-url";
import { getSessionSnapshot } from "../lib/session";

const MULTIPART_THRESHOLD_BYTES = 5 * 1024 * 1024;

type UploadPanelProps = {
  currentPath: string;
  onUploaded: () => Promise<void> | void;
};

type UploadMeta = {
  relativePath?: string;
};

type UploadBody = Record<string, never>;

type UploadRuntime = {
  currentPath: string;
};

type ConflictDialogState = {
  items: GithubComAbhishekPenDriveBackendInternalApiDtoDuplicatePreviewItem[];
  impactedPaths: string[];
};

type UploadConflictPolicy = Exclude<
  GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy,
  "reject"
>;

function debugUpload(event: string, details?: Record<string, unknown>) {
  if (!import.meta.env.DEV) {
    return;
  }

  console.debug(`[upload-panel] ${event}`, details ?? {});
}

export function UploadPanel({
  currentPath,
  onUploaded,
}: UploadPanelProps) {
  const runtimeRef = useRef<UploadRuntime>({ currentPath });
  const folderInputRef = useRef<HTMLInputElement | null>(null);
  const uppyConfiguredRef = useRef(false);
  const pendingConflictResolverRef = useRef<
    ((policy: UploadConflictPolicy | null) => void) | null
  >(null);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [conflictDialog, setConflictDialog] = useState<ConflictDialogState | null>(
    null,
  );
  const [uppy] = useState(
    () =>
      new Uppy<UploadMeta, UploadBody>({
        autoProceed: false,
        allowMultipleUploadBatches: true,
      }),
  );

  function requestConflictPolicy(
    value: ConflictDialogState,
  ): Promise<UploadConflictPolicy | null> {
    setConflictDialog(value);
    return new Promise((resolve) => {
      pendingConflictResolverRef.current = resolve;
    });
  }

  function resolveConflictDecision(policy: UploadConflictPolicy | null) {
    pendingConflictResolverRef.current?.(policy);
    pendingConflictResolverRef.current = null;
    setConflictDialog(null);
  }

  useEffect(() => {
    runtimeRef.current = { currentPath };
  }, [currentPath]);

  useEffect(() => {
    debugUpload("configure-uppy", {
      currentPath,
    });

    if (!uppyConfiguredRef.current) {
      configureUppy({
        requestConflictPolicy: requestConflictPolicy,
        setError,
        uppy,
        runtimeRef,
      });
      uppyConfiguredRef.current = true;
    }

    return () => {
      uppy.destroy();
    };
  }, [currentPath, setError, uppy]);

  useEffect(() => {
    const detachHandlers = attachCompletionHandlers({
      onUploaded,
      setError,
      setMessage,
      uppy,
    });

    return () => {
      detachHandlers();
    };
  }, [onUploaded, uppy]);

  function handleFolderSelection(event: ChangeEvent<HTMLInputElement>) {
    const files = event.currentTarget.files;
    if (!files) {
      return;
    }

    setError(null);
    setMessage(null);

    debugUpload("folder-selection", {
      count: files.length,
      currentPath,
      rawPaths: Array.from(files).map((file) => file.webkitRelativePath || file.name),
    });

    for (const file of Array.from(files)) {
      const relativePath = normalizeFolderRelativePath(file.webkitRelativePath) || file.name;
      debugUpload("folder-file-queued", {
        fileName: file.name,
        normalizedRelativePath: relativePath,
        rawRelativePath: file.webkitRelativePath || file.name,
      });
      try {
        uppy.addFile({
          name: file.name,
          type: file.type,
          data: file,
          source: "Local",
          meta: {
            relativePath,
          },
        });
      } catch (error) {
        setError(
          error instanceof Error ? error.message : "failed to queue folder file",
        );
      }
    }

    event.currentTarget.value = "";
  }

  return (
    <>
      <UploadCard
        currentPath={currentPath}
        error={error}
        message={message}
        onPickFolder={() => folderInputRef.current?.click()}
        uppy={uppy}
      />
      <input
        className="sr-only"
        multiple
        onChange={handleFolderSelection}
        ref={folderInputRef}
        type="file"
        {...({
          directory: "",
          webkitdirectory: "",
        } as Record<string, string>)}
      />
      {conflictDialog ? (
        <ConflictPreviewDialog
          conflictDialog={conflictDialog}
          onCancel={() => resolveConflictDecision(null)}
          onSelect={(policy) => resolveConflictDecision(policy)}
        />
      ) : null}
    </>
  );
}

function UploadCard({
  currentPath,
  error,
  message,
  onPickFolder,
  uppy,
}: {
  currentPath: string;
  error: string | null;
  message: string | null;
  onPickFolder: () => void;
  uppy: Uppy<UploadMeta, UploadBody>;
}) {
  return (
    <Card className="flex flex-col gap-2">
      <CardHeader>
        <CardTitle>Upload files or folders</CardTitle>
        <CardDescription>
          Drop files onto the target, click the surface to browse files, or pick a
          folder to preserve nested paths under <code>{currentPath || "root"}</code>.
          Files above 5 MB switch to multipart upload automatically.
        </CardDescription>
      </CardHeader>
      <div className="flex flex-wrap gap-2 px-6">
        <Button variant="outline" onClick={onPickFolder} type="button">
          Pick folder
        </Button>
        <Button
          variant="outline"
          onClick={() => uppy.cancelAll()}
          type="button"
        >
          Clear queue
        </Button>
      </div>
      <UppySurface
        description="Drop files here, click to add files, or use Pick folder to keep nested paths."
        uppy={uppy}
      />
      {message ? (
        <p className="text-sm text-muted-foreground px-6 pb-2">{message}</p>
      ) : null}
      {error ? (
        <p className="text-sm text-muted-foreground text-destructive px-6 pb-2">{error}</p>
      ) : null}
    </Card>
  );
}

function UppySurface({
  description,
  uppy,
}: {
  description: string;
  uppy: Uppy<UploadMeta, UploadBody>;
}) {
  const [nextStepHint, setNextStepHint] = useState<string | null>(null);
  const files = useUppyState(uppy, (state) => Object.values(state.files)) as Array<
    UppyFile<UploadMeta, UploadBody>
  >;
  const fileCount = files.length;
  const totalBytes = files.reduce(
    (sum, file) => sum + (typeof file.size === "number" ? file.size : 0),
    0,
  );
  const uploadedBytes = files.reduce(
    (sum, file) => sum + getUploadedBytes(file),
    0,
  );
  const aggregateProgress = totalBytes > 0 ? Math.round((uploadedBytes / totalBytes) * 100) : 0;
  const visibleNextStepHint = fileCount === 0 ? nextStepHint : null;
  const shouldHighlightDropzone = visibleNextStepHint !== null;

  async function handleUploadClick() {
    if (fileCount === 0) {
      setNextStepHint("Add files first by dropping them here, clicking the surface, or picking a folder.");
      return;
    }

    setNextStepHint(null);
    await uppy.upload();
  }

  return (
    <UppyContextProvider uppy={uppy}>
      <div className="flex flex-col gap-2 px-6 pb-6">
        <div
          className={`rounded-xl transition-all ${
            shouldHighlightDropzone
              ? "ring-2 ring-primary ring-offset-2 ring-offset-background"
              : ""
          }`}
        >
          <Dropzone height="180px" note={description} width="100%" />
        </div>
        <div className="flex items-center justify-between">
          <div className="min-w-0">
            <p className="text-sm text-muted-foreground">{fileCount} queued</p>
            {visibleNextStepHint ? (
              <p className="mt-1 text-xs text-primary">{visibleNextStepHint}</p>
            ) : (
              <p className="mt-1 text-xs text-muted-foreground">
                {fileCount === 0
                  ? "Next step: add files or pick a folder."
                  : "Next step: review the queue, then upload."}
              </p>
            )}
          </div>
          <Button onClick={() => void handleUploadClick()} type="button">
            Upload queue
          </Button>
        </div>
        {fileCount > 0 ? (
          <>
            <Progress value={aggregateProgress} />
            <div className="rounded-md border divide-y">
              {files.map((file) => {
                const label = file.meta.relativePath || file.name;
                const percentage =
                  typeof file.progress?.percentage === "number"
                    ? file.progress.percentage
                    : 0;

                return (
                  <div className="space-y-2 p-3" key={file.id}>
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <p className="truncate text-sm font-medium">{label}</p>
                          <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
                            {isFolderUpload(file) ? "folder" : "file"}
                          </span>
                        </div>
                        <p className="text-xs text-muted-foreground">
                          {renderFileStatus(file)}
                        </p>
                      </div>
                      <p className="shrink-0 text-xs text-muted-foreground">
                        {formatBytes(getUploadedBytes(file))}
                        {typeof file.size === "number"
                          ? ` / ${formatBytes(file.size)}`
                          : ""}
                      </p>
                    </div>
                    <Progress value={percentage} />
                  </div>
                );
              })}
            </div>
          </>
        ) : null}
      </div>
    </UppyContextProvider>
  );
}

function configureUppy({
  requestConflictPolicy,
  runtimeRef,
  setError,
  uppy,
}: {
  requestConflictPolicy: (
    value: ConflictDialogState,
  ) => Promise<UploadConflictPolicy | null>;
  runtimeRef: MutableRefObject<UploadRuntime>;
  setError: (message: string | null) => void;
  uppy: Uppy<UploadMeta, UploadBody>;
}) {
  uppy.on("file-added", (file) => {
    const nativeFile = file.data;
    if (!(nativeFile instanceof File)) {
      return;
    }

    const relativePath =
      file.meta.relativePath ||
      normalizeFolderRelativePath(nativeFile.webkitRelativePath);
    debugUpload("uppy-file-added", {
      fileId: file.id,
      isFolderItem: Boolean(relativePath),
      name: file.name,
      relativePath: relativePath || null,
    });
    if (!relativePath) {
      return;
    }

    uppy.setFileMeta(file.id, {
      relativePath,
    });
  });

  uppy.addUploader(async (fileIDs) => {
    setError(null);
    const files = fileIDs
      .map((fileID) => uppy.getFile(fileID))
      .filter((file): file is UppyFile<UploadMeta, UploadBody> => Boolean(file));

    debugUpload("uploader-start", {
      fileIDs,
      files: files.map((file) => ({
        id: file.id,
        name: file.name,
        relativePath: file.meta.relativePath || null,
        size: file.size ?? null,
      })),
    });

    const runtime = runtimeRef.current;
    const preview = await previewBatchConflicts(runtime, files);
    debugUpload("preview-result", {
      hasConflicts: preview?.has_conflicts ?? null,
      impactedPaths: preview?.impacted_paths ?? [],
      items: preview?.items ?? [],
    });
    const conflictPolicy = preview?.has_conflicts
      ? await requestConflictPolicy({
          impactedPaths: preview.impacted_paths ?? [],
          items: preview.items ?? [],
        })
      : null;

    if (preview?.has_conflicts && !conflictPolicy) {
      setError("upload cancelled");
      return;
    }

    const normalizedPolicy =
      conflictPolicy ?? DuplicateConflictPolicy.DUPLICATE_CONFLICT_POLICY_REJECT;

    const folderBatchFiles = files.filter(
      (file) => isFolderUpload(file) && !isMultipartEligible(file),
    );

    if (folderBatchFiles.length > 0) {
      try {
        await uploadFolderBatch(
          runtime,
          folderBatchFiles,
          normalizedPolicy,
          (progressByFile) => {
            for (const [fileId, progress] of progressByFile.entries()) {
              updateFileProgress(
                uppy,
                fileId,
                progress.bytesUploaded,
                progress.bytesTotal,
              );
            }
          },
        );

        for (const file of folderBatchFiles) {
          markFileComplete(uppy, file.id);
          const refreshed = uppy.getFile(file.id);
          if (refreshed) {
            uppy.emit("upload-success", refreshed, {
              body: undefined,
              status: 201,
              uploadURL: undefined,
            });
          }
        }
      } catch (error) {
        for (const file of folderBatchFiles) {
          const refreshed = uppy.getFile(file.id);
          if (!refreshed) {
            continue;
          }
          uppy.emit(
            "upload-error",
            refreshed,
            error instanceof Error ? error : new Error("upload failed"),
          );
        }
        return;
      }
    }

    const remainingFiles = files.filter(
      (file) => !folderBatchFiles.some((batchFile) => batchFile.id === file.id),
    );

    for (const file of remainingFiles) {
      await uploadOneFile({
        conflictPolicy: normalizedPolicy,
        file,
        runtime,
        uppy,
      });
    }
  });
}

function attachCompletionHandlers({
  onUploaded,
  setError,
  setMessage,
  uppy,
}: {
  onUploaded: () => Promise<void> | void;
  setError: (message: string | null) => void;
  setMessage: (message: string | null) => void;
  uppy: Uppy<UploadMeta, UploadBody>;
}) {
  const handleComplete = async (result: UploadResult<UploadMeta, UploadBody>) => {
    const successful = result.successful ?? [];
    const failed = result.failed ?? [];

    if (successful.length > 0) {
      await onUploaded();
      setMessage(summarizeUploadResult(successful));
      for (const file of successful) {
        uppy.removeFile(file.id);
      }
    }

    if (failed.length > 0) {
      setError(failed[0].error || "failed to upload selection");
      return;
    }

    setError(null);
  };

  uppy.on("complete", handleComplete);
  return () => {
    uppy.off("complete", handleComplete);
  };
}

function resolveUploadTarget(
  currentPath: string,
  file: UppyFile<UploadMeta, UploadBody>,
) {
  if (!isFolderUpload(file)) {
    return {
      filename: file.name,
      path: currentPath,
    };
  }

  const relativePath = file.meta.relativePath || file.name;
  const normalized = relativePath
    .split("/")
    .filter(Boolean);
  const filename = normalized.at(-1) || file.name;
  const parentSegments = normalized.slice(0, -1);
  const pathSegments = [currentPath, parentSegments.join("/")]
    .filter(Boolean)
    .join("/")
    .replaceAll("//", "/");

  return {
    filename,
    path: pathSegments,
  };
}

function isMultipartEligible(file: UppyFile<UploadMeta, UploadBody>) {
  return typeof file.size === "number" && file.size > MULTIPART_THRESHOLD_BYTES;
}

function isFolderUpload(file: UppyFile<UploadMeta, UploadBody>) {
  return Boolean(file.meta.relativePath);
}

function summarizeUploadResult(files: UppyFile<UploadMeta, UploadBody>[]) {
  const folderCount = files.filter((file) => isFolderUpload(file)).length;
  const fileCount = files.length - folderCount;
  const parts: string[] = [];

  if (fileCount > 0) {
    parts.push(`${fileCount} file${fileCount === 1 ? "" : "s"}`);
  }

  if (folderCount > 0) {
    parts.push(`${folderCount} folder item${folderCount === 1 ? "" : "s"}`);
  }

  if (parts.length === 0) {
    return null;
  }

  return `${parts.join(" and ")} uploaded`;
}

function normalizeFolderRelativePath(relativePath: string) {
  const normalized = relativePath
    .replaceAll("\\", "/")
    .split("/")
    .filter(Boolean);

  return normalized.join("/");
}

function authHeaders(): { Authorization: string } {
  const session = getSessionSnapshot();
  if (!session?.accessToken) {
    throw new Error("missing access token");
  }

  return { Authorization: `Bearer ${session.accessToken}` };
}

async function uploadViaSingleRequest(
  target: {
    filename: string;
    path: string;
  },  file: UppyFile<UploadMeta, UploadBody>,
  conflictPolicy: GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy,
  onProgress?: (bytesUploaded: number, bytesTotal: number) => void,
): Promise<UploadBody | undefined> {
  if (!(file.data instanceof Blob)) {
    throw new Error("file data is unavailable");
  }

  const body: PostApiV1FilesUploadData["body"] = {
    file: new File([file.data], file.name, { type: file.type }),
  };
  if (target.path) {
    body.path = target.path;
  }
  if (target.filename !== file.name) {
    body.filename = target.filename;
  }
  body.conflict_policy = conflictPolicy;

  return uploadFormData<UploadBody | undefined>({
    body,
    headers: authHeaders(),
    onProgress,
    url: "/api/v1/files/upload",
  });
}

async function uploadViaMultipart(
  target: {
    filename: string;
    path: string;
  },  file: UppyFile<UploadMeta, UploadBody>,
  conflictPolicy: GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy,
  onProgress?: (bytesUploaded: number, bytesTotal: number) => void,
): Promise<UploadBody | undefined> {
  if (!(file.data instanceof Blob) || typeof file.size !== "number") {
    throw new Error("file data is unavailable");
  }
  const fileData = file.data;
  const totalSize = file.size;

  const { data: initiateData, error: initiateError, response: initiateResponse } =
    await postApiV1FilesUploadMultipartInitiate({
      client: apiClient,
      body: {
        content_type: file.type,
        conflict_policy: conflictPolicy,
        filename: target.filename,
        path: target.path,
        size: file.size,
      },
      headers: authHeaders(),
    });
  if (initiateError) {
    throw new Error(getErrorMessage(initiateError, initiateResponse.status));
  }
  const initiatePayload = requireMultipartSession(initiateData);

  const uploadedParts: Array<{ etag: string; part_number: number }> = [];

  try {
    let partNumber = 1;
    let uploadedBytes = 0;
    for (
      let offset = 0;
      offset < fileData.size;
      offset += initiatePayload.part_size
    ) {
      const chunk = fileData.slice(offset, offset + initiatePayload.part_size);
      const partData = await uploadMultipartPart({
        chunk,
        key: initiatePayload.key,
        partNumber,
        uploadId: initiatePayload.upload_id,
        onProgress: (chunkBytesUploaded) => {
          onProgress?.(uploadedBytes + chunkBytesUploaded, totalSize);
        },
        filename: target.filename,
      });
      const partPayload = requireMultipartPart(partData);

      uploadedParts.push({
        etag: partPayload.etag,
        part_number: partPayload.part_number,
      });
      uploadedBytes += chunk.size;
      onProgress?.(uploadedBytes, totalSize);
      partNumber += 1;
    }

    const { data: completeData, error: completeError, response: completeResponse } =
      await postApiV1FilesUploadMultipartComplete({
        client: apiClient,
        body: {
          key: initiatePayload.key,
          parts: uploadedParts,
          upload_id: initiatePayload.upload_id,
        },
        headers: authHeaders(),
      });
    if (completeError) {
      throw new Error(getErrorMessage(completeError, completeResponse.status));
    }

    return completeData as UploadBody | undefined;
  } catch (error) {
    await abortMultipartUpload(initiatePayload);
    throw error;
  }
}

async function abortMultipartUpload(
  multipart: GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadInitiateResponse,
) {
  await postApiV1FilesUploadMultipartAbort({
    client: apiClient,    body: {
      key: multipart.key,
      upload_id: multipart.upload_id,
    },
    headers: authHeaders(),
  });
}

async function previewBatchConflicts(
  runtime: UploadRuntime,
  files: UppyFile<UploadMeta, UploadBody>[],
): Promise<GithubComAbhishekPenDriveBackendInternalApiDtoDuplicatePreviewResponse | null> {
  if (files.length === 0) {
    return null;
  }

  const relativePaths = files.map((file) => file.meta.relativePath || file.name);

  debugUpload("preview-request", {
    currentPath: runtime.currentPath,
    relativePaths,
  });

  const { data, error, response } = await postApiV1FilesDuplicatesPreview({
    client: apiClient,
    body: {
      path: runtime.currentPath,
      relative_paths: relativePaths,
    },
    headers: authHeaders(),
  });
  if (error) throw new Error(getErrorMessage(error, response.status));

  return data ?? null;
}

async function uploadFolderBatch(
  runtime: UploadRuntime,
  files: UppyFile<UploadMeta, UploadBody>[],
  conflictPolicy: GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy,
  onProgress?: (
    progressByFile: Map<string, { bytesUploaded: number; bytesTotal: number }>,
  ) => void,
): Promise<GithubComAbhishekPenDriveBackendInternalApiDtoFolderUploadResponse | undefined> {
  const batchFiles = files.map((file) => {
    if (!(file.data instanceof Blob)) {
      throw new Error("file data is unavailable");
    }

    return {
      blob: new File([file.data], file.name, { type: file.type }),
      file,
      relativePath: file.meta.relativePath || file.name,
      size: typeof file.size === "number" ? file.size : 0,
    };
  });

  return uploadFormData<GithubComAbhishekPenDriveBackendInternalApiDtoFolderUploadResponse | undefined>(
    {
      body: {
        conflict_policy: conflictPolicy,
        files: batchFiles.map(({ blob }) => blob),
        path: runtime.currentPath || undefined,
        relative_paths: batchFiles.map(({ relativePath }) => relativePath),
      },
      headers: authHeaders(),
      onProgress: (bytesUploaded) => {
        onProgress?.(distributeBatchProgress(files, bytesUploaded));
      },
      totalBytes: batchFiles.reduce((sum, item) => sum + item.size, 0),
      url: "/api/v1/files/upload-folder",
    },
  );
}

function getErrorMessage(payload: unknown, status: number) {
  if (
    payload &&
    typeof payload === "object" &&
    "error" in payload &&
    payload.error &&
    typeof payload.error === "object" &&
    "message" in payload.error &&
    typeof payload.error.message === "string"
  ) {
    return payload.error.message;
  }

  return `upload failed with status ${status}`;
}

async function uploadOneFile({
  conflictPolicy,
  file,
  runtime,
  uppy,
}: {
  conflictPolicy: GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy;
  file: UppyFile<UploadMeta, UploadBody>;
  runtime: UploadRuntime;
  uppy: Uppy<UploadMeta, UploadBody>;
}) {
  try {
    const target = resolveUploadTarget(
      runtime.currentPath,
      file,
    );
    const responseBody = isMultipartEligible(file)
      ? await uploadViaMultipart(target, file, conflictPolicy, (bytesUploaded, bytesTotal) =>
          updateFileProgress(uppy, file.id, bytesUploaded, bytesTotal),
        )      : await uploadViaSingleRequest(target, file, conflictPolicy, (bytesUploaded, bytesTotal) =>
          updateFileProgress(uppy, file.id, bytesUploaded, bytesTotal),
        );
    markFileComplete(uppy, file.id);
    const refreshed = uppy.getFile(file.id);
    if (!refreshed) {
      return;
    }

    uppy.emit("upload-success", refreshed, {
      body: responseBody,
      status: 201,
      uploadURL: undefined,
    });
  } catch (error) {
    const refreshed = uppy.getFile(file.id);
    if (!refreshed) {
      return;
    }
    uppy.emit(
      "upload-error",
      refreshed,
      error instanceof Error ? error : new Error("upload failed"),
    );
  }
}

function updateFileProgress(
  uppy: Uppy<UploadMeta, UploadBody>,
  fileId: string,
  bytesUploaded: number,
  bytesTotal: number,
) {
  const file = uppy.getFile(fileId);
  if (!file) {
    return;
  }

  const safeTotal =
    bytesTotal > 0 ? bytesTotal : typeof file.size === "number" ? file.size : 0;
  const safeUploaded = Math.max(0, Math.min(bytesUploaded, safeTotal || bytesUploaded));
  const percentage =
    safeTotal > 0 ? Math.min(100, Math.round((safeUploaded / safeTotal) * 100)) : 0;
  const progress = {
    bytesTotal: safeTotal,
    bytesUploaded: safeUploaded,
    percentage,
    uploadComplete: safeTotal > 0 && safeUploaded >= safeTotal,
    uploadStarted: file.progress?.uploadStarted ?? Date.now(),
  };

  uppy.setFileState(fileId, { progress });
  const refreshed = uppy.getFile(fileId);
  if (refreshed) {
    uppy.emit("upload-progress", refreshed, progress);
  }
}

function markFileComplete(uppy: Uppy<UploadMeta, UploadBody>, fileId: string) {
  const file = uppy.getFile(fileId);
  if (!file) {
    return;
  }

  updateFileProgress(
    uppy,
    fileId,
    typeof file.size === "number" ? file.size : 0,
    typeof file.size === "number" ? file.size : 0,
  );
}

function distributeBatchProgress(
  files: UppyFile<UploadMeta, UploadBody>[],
  bytesUploaded: number,
) {
  const progressByFile = new Map<string, { bytesUploaded: number; bytesTotal: number }>();
  let remaining = bytesUploaded;

  for (const file of files) {
    const bytesTotal = typeof file.size === "number" ? file.size : 0;
    const uploadedForFile = Math.max(0, Math.min(remaining, bytesTotal));
    progressByFile.set(file.id, {
      bytesUploaded: uploadedForFile,
      bytesTotal,
    });
    remaining -= uploadedForFile;
  }

  return progressByFile;
}

async function uploadMultipartPart({
  chunk,
  filename,
  key,
  partNumber,
  uploadId,
  onProgress,
}: {
  chunk: Blob;
  filename: string;
  key: string;
  partNumber: number;
  uploadId: string;
  onProgress?: (bytesUploaded: number) => void;
}): Promise<GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadPartResponse | undefined> {
  return uploadFormData<GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadPartResponse | undefined>(
    {
      body: {
        key,
        part: new File([chunk], `${filename}.part-${partNumber}`, {
          type: "application/octet-stream",
        }),
        part_number: String(partNumber),
        upload_id: uploadId,
      },
      headers: authHeaders(),
      onProgress: (bytesUploaded) => {
        onProgress?.(bytesUploaded);
      },
      totalBytes: chunk.size,
      url: "/api/v1/files/upload-multipart/part",
    },
  );
}

async function uploadFormData<T>({
  body,
  headers,
  onProgress,
  totalBytes,
  url,
}: {
  body: Record<string, Blob | File | string | string[] | File[] | Blob[] | undefined>;
  headers: Record<string, string>;
  onProgress?: (bytesUploaded: number, bytesTotal: number) => void;
  totalBytes?: number;
  url: string;
}): Promise<T> {
  const formData = new FormData();
  for (const [key, value] of Object.entries(body)) {
    if (value === undefined || value === null) {
      continue;
    }

    if (Array.isArray(value)) {
      for (const item of value) {
        formData.append(key, item);
      }
      continue;
    }

    formData.append(key, value);
  }

  return xhrJsonRequest<T>({
    body: formData,
    headers,
    method: "POST",
    onProgress,
    totalBytes,
    url,
  });
}

async function xhrJsonRequest<T>({
  body,
  headers,
  method,
  onProgress,
  totalBytes,
  url,
}: {
  body: Document | XMLHttpRequestBodyInit | null;
  headers?: Record<string, string>;
  method: string;
  onProgress?: (bytesUploaded: number, bytesTotal: number) => void;
  totalBytes?: number;
  url: string;
}): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open(method, `${API_BASE_URL}${url}`);
    request.responseType = "text";

    for (const [key, value] of Object.entries(headers ?? {})) {
      request.setRequestHeader(key, value);
    }

    if (onProgress) {
      request.upload.onprogress = (event) => {
        const bytesTotalValue =
          totalBytes ?? (event.lengthComputable ? event.total : 0);
        const bytesUploadedValue =
          bytesTotalValue > 0
            ? Math.min(event.loaded, bytesTotalValue)
            : event.loaded;
        onProgress(bytesUploadedValue, bytesTotalValue);
      };
    }

    request.onerror = () => {
      reject(new Error("network request failed"));
    };

    request.onload = () => {
      const raw = request.responseText;
      const payload = raw ? (JSON.parse(raw) as unknown) : undefined;

      if (request.status >= 200 && request.status < 300) {
        if (onProgress && typeof totalBytes === "number") {
          onProgress(totalBytes, totalBytes);
        }
        resolve(payload as T);
        return;
      }

      reject(new Error(getErrorMessage(payload, request.status)));
    };

    request.send(body);
  });
}

function requireMultipartSession(
  payload: GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadInitiateResponse | undefined,
) {
  if (
    !payload ||
    !payload.key ||
    !payload.upload_id ||
    typeof payload.part_size !== "number" ||
    !payload.name
  ) {
    throw new Error("multipart initiate response is incomplete");
  }

  return payload as Required<GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadInitiateResponse>;
}

function requireMultipartPart(
  payload: GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadPartResponse | undefined,
) {
  if (!payload || !payload.etag || typeof payload.part_number !== "number") {
    throw new Error("multipart part response is incomplete");
  }

  return payload as Required<GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadPartResponse>;
}

function ConflictPreviewDialog({
  conflictDialog,
  onCancel,
  onSelect,
}: {
  conflictDialog: ConflictDialogState;
  onCancel: () => void;
  onSelect: (policy: UploadConflictPolicy) => void;
}) {
  return (
    <Dialog open={true} onOpenChange={(open) => !open && onCancel()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Conflicts found in this upload</DialogTitle>
          <DialogDescription>
            Choose whether to keep both versions or replace the existing files.
          </DialogDescription>
        </DialogHeader>
        <div className="max-h-[60vh] overflow-y-auto rounded-lg border">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="sticky top-0 bg-background">
              <tr className="border-b">
                <th className="px-4 py-3 font-semibold">Existing file</th>
                <th className="px-4 py-3 font-semibold">Renamed copy target</th>
              </tr>
            </thead>
            <tbody>
              {conflictDialog.items
                .filter((item) => item.conflict)
                .map((item, index) => (
                  <tr
                    className="border-b align-top last:border-b-0"
                    key={item.requested_path || item.rename_path || `conflict-${index}`}
                  >
                    <td className="px-4 py-3 break-all font-medium">
                      {item.existing_path || item.requested_path || "unknown"}
                    </td>
                    <td className="px-4 py-3 break-all text-muted-foreground">
                      {item.rename_path || "not available"}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
        <DialogFooter className="gap-2 sm:justify-between sm:space-x-0">
          <Button className="w-full sm:w-auto" variant="outline" onClick={onCancel}>
            Cancel upload
          </Button>
          <Button
            className="w-full sm:w-auto"
            variant="outline"
            onClick={() => onSelect(DuplicateConflictPolicy.DUPLICATE_CONFLICT_POLICY_RENAME)}
          >
            Create renamed copies
          </Button>
          <Button
            className="w-full sm:w-auto"
            onClick={() => onSelect(DuplicateConflictPolicy.DUPLICATE_CONFLICT_POLICY_REPLACE)}
          >
            Replace existing files
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function renderFileStatus(file: UppyFile<UploadMeta, UploadBody>): ReactNode {
  if (file.error) {
    return <span className="text-destructive">{file.error}</span>;
  }

  if (file.progress?.uploadComplete) {
    return "Uploaded";
  }

  if (file.progress?.uploadStarted) {
    return `${Math.round(file.progress.percentage ?? 0)}% uploaded`;
  }

  return "Queued";
}

function getUploadedBytes(file: UppyFile<UploadMeta, UploadBody>) {
  return typeof file.progress?.bytesUploaded === "number"
    ? file.progress.bytesUploaded
    : 0;
}
