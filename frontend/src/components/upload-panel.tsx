import Uppy, { type UploadResult, type UppyFile } from "@uppy/core";
import {
  Dropzone,
  UploadButton,
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
  DialogFooter,
} from "@/components/ui/dialog";
import { Progress } from "@/components/ui/progress";

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

const MULTIPART_THRESHOLD_BYTES = 5 * 1024 * 1024;
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://127.0.0.1:8080";

type UploadPanelProps = {
  accessToken: string;
  currentPath: string;
  onUploaded: () => Promise<void> | void;
};

type UploadMeta = {
  relativePath?: string;
};

type UploadBody = Record<string, never>;

type UploadMode = "file" | "folder";

type UploadRuntime = {
  accessToken: string;
  currentPath: string;
};

type ConflictDialogState = {
  items: GithubComAbhishekPenDriveBackendInternalApiDtoDuplicatePreviewItem[];
  impactedPaths: string[];
  mode: UploadMode;
};

type UploadConflictPolicy = Exclude<
  GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy,
  "reject"
>;

export function UploadPanel({
  accessToken,
  currentPath,
  onUploaded,
}: UploadPanelProps) {
  const runtimeRef = useRef<UploadRuntime>({ accessToken, currentPath });
  const folderInputRef = useRef<HTMLInputElement | null>(null);
  const pendingConflictResolverRef = useRef<
    ((policy: UploadConflictPolicy | null) => void) | null
  >(null);
  const [fileMessage, setFileMessage] = useState<string | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [folderMessage, setFolderMessage] = useState<string | null>(null);
  const [folderError, setFolderError] = useState<string | null>(null);
  const [conflictDialog, setConflictDialog] = useState<ConflictDialogState | null>(
    null,
  );
  const [fileUppy] = useState(
    () =>
      new Uppy<UploadMeta, UploadBody>({
        autoProceed: false,
        allowMultipleUploadBatches: true,
      }),
  );
  const [folderUppy] = useState(
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
    runtimeRef.current = { accessToken, currentPath };
  }, [accessToken, currentPath]);

  useEffect(() => {
    configureUppy({
      mode: "file",
      requestConflictPolicy: requestConflictPolicy,
      setError: setFileError,
      uppy: fileUppy,
      runtimeRef,
    });
    configureUppy({
      mode: "folder",
      requestConflictPolicy: requestConflictPolicy,
      setError: setFolderError,
      uppy: folderUppy,
      runtimeRef,
    });

    return () => {
      fileUppy.destroy();
      folderUppy.destroy();
    };
  }, [fileUppy, folderUppy]);

  useEffect(() => {
    const detachFileHandlers = attachCompletionHandlers({
      label: "file",
      onUploaded,
      setError: setFileError,
      setMessage: setFileMessage,
      uppy: fileUppy,
    });
    const detachFolderHandlers = attachCompletionHandlers({
      label: "folder item",
      onUploaded,
      setError: setFolderError,
      setMessage: setFolderMessage,
      uppy: folderUppy,
    });

    return () => {
      detachFileHandlers();
      detachFolderHandlers();
    };
  }, [fileUppy, folderUppy, onUploaded]);

  function handleFolderSelection(event: ChangeEvent<HTMLInputElement>) {
    const files = event.currentTarget.files;
    if (!files) {
      return;
    }

    setFolderError(null);
    setFolderMessage(null);

    for (const file of Array.from(files)) {
      const relativePath = file.webkitRelativePath || file.name;
      try {
        folderUppy.addFile({
          name: file.name,
          type: file.type,
          data: file,
          source: "Local",
          meta: {
            relativePath,
          },
        });
      } catch (error) {
        setFolderError(
          error instanceof Error ? error.message : "failed to queue folder file",
        );
      }
    }

    event.currentTarget.value = "";
  }

  return (
    <>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <UploadCard
          description="Drag files onto the target or browse from this device. Files above 5 MB switch to multipart upload automatically."
          error={fileError}
          message={fileMessage}
          title="Quick file upload"
          uppy={fileUppy}
        />
        <Card className="flex flex-col gap-2">
          <CardHeader>
            <CardTitle>Preserve nested paths</CardTitle>
            <CardDescription>
              Pick a folder from disk to preserve nested paths under{" "}
              <code>{currentPath || "root"}</code>. Files above 5 MB switch to
              multipart upload automatically.
            </CardDescription>
          </CardHeader>
          <div className="flex gap-2 px-6">
            <Button
              variant="outline"
              onClick={() => folderInputRef.current?.click()}
              type="button"
            >
              Pick folder
            </Button>
            <Button
              variant="outline"
              onClick={() => folderUppy.cancelAll()}
              type="button"
            >
              Clear queue
            </Button>
          </div>
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
          <UppySurface
            description="Drop files here or pick a folder using the button above."
            mode="folder"
            uppy={folderUppy}
          />
          {folderMessage ? (
            <p className="text-sm text-muted-foreground px-6 pb-2">{folderMessage}</p>
          ) : null}
          {folderError ? (
            <p className="text-sm text-muted-foreground text-destructive px-6 pb-2">{folderError}</p>
          ) : null}
        </Card>
      </div>
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
  description,
  error,
  message,
  title,
  uppy,
}: {
  description: string;
  error: string | null;
  message: string | null;
  title: string;
  uppy: Uppy<UploadMeta, UploadBody>;
}) {
  return (
    <Card className="flex flex-col gap-2">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <div className="flex gap-2 px-6">
        <Button
          variant="outline"
          onClick={() => uppy.cancelAll()}
          type="button"
        >
          Clear queue
        </Button>
      </div>
      <UppySurface
        description="Drag files here or click the surface to browse."
        mode="file"
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
  mode,
  uppy,
}: {
  description: string;
  mode: UploadMode;
  uppy: Uppy<UploadMeta, UploadBody>;
}) {
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

  return (
    <UppyContextProvider uppy={uppy}>
      <div className="flex flex-col gap-2 px-6 pb-6">
        <Dropzone height="180px" note={description} width="100%" />
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">{fileCount} queued</p>
          <UploadButton />
        </div>
        {fileCount > 0 ? (
          <>
            <Progress value={aggregateProgress} />
            <div className="rounded-md border divide-y">
              {files.map((file) => {
                const label =
                  mode === "folder" ? file.meta.relativePath || file.name : file.name;
                const percentage =
                  typeof file.progress?.percentage === "number"
                    ? file.progress.percentage
                    : 0;

                return (
                  <div className="space-y-2 p-3" key={file.id}>
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-medium">{label}</p>
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
  mode,
  requestConflictPolicy,
  runtimeRef,
  setError,
  uppy,
}: {
  mode: UploadMode;
  requestConflictPolicy: (
    value: ConflictDialogState,
  ) => Promise<UploadConflictPolicy | null>;
  runtimeRef: MutableRefObject<UploadRuntime>;
  setError: (message: string | null) => void;
  uppy: Uppy<UploadMeta, UploadBody>;
}) {
  if (mode === "folder") {
    uppy.on("file-added", (file) => {
      const nativeFile = file.data;
      if (!(nativeFile instanceof File)) {
        return;
      }

      const relativePath = file.meta.relativePath || nativeFile.webkitRelativePath;
      uppy.setFileMeta(file.id, {
        relativePath: relativePath || file.name,
      });
    });
  }

  uppy.addUploader(async (fileIDs) => {
    setError(null);
    const files = fileIDs
      .map((fileID) => uppy.getFile(fileID))
      .filter((file): file is UppyFile<UploadMeta, UploadBody> => Boolean(file));

    const runtime = runtimeRef.current;
    const preview = await previewBatchConflicts(runtime, mode, files);
    const conflictPolicy = preview?.has_conflicts
      ? await requestConflictPolicy({
          impactedPaths: preview.impacted_paths ?? [],
          items: preview.items ?? [],
          mode,
        })
      : null;

    if (preview?.has_conflicts && !conflictPolicy) {
      throw new Error("upload cancelled");
    }

    const normalizedPolicy =
      conflictPolicy ?? DuplicateConflictPolicy.DUPLICATE_CONFLICT_POLICY_REJECT;

    if (mode === "folder") {
      const batchFiles = files.filter((file) => !isMultipartEligible(file));
      const multipartFiles = files.filter((file) => isMultipartEligible(file));

      if (batchFiles.length > 0) {
        try {
          await uploadFolderBatch(
            runtime,
            batchFiles,
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

          for (const file of batchFiles) {
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
          for (const file of batchFiles) {
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

      for (const file of multipartFiles) {
        await uploadOneFile({
          conflictPolicy: normalizedPolicy,
          file,
          runtime,
          uppy,
        });
      }
      return;
    }

    for (const file of files) {
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
  label,
  onUploaded,
  setError,
  setMessage,
  uppy,
}: {
  label: string;
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
      setMessage(
        `${successful.length} ${label}${successful.length === 1 ? "" : "s"} uploaded`,
      );
      for (const file of successful) {
        uppy.removeFile(file.id);
      }
    }

    if (failed.length > 0) {
      setError(failed[0].error || `failed to upload ${label}`);
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
  mode: UploadMode,
  currentPath: string,
  file: UppyFile<UploadMeta, UploadBody>,
) {
  if (mode === "file") {
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

function authHeaders(token: string): { Authorization: string } {
  return { Authorization: `Bearer ${token}` };
}

async function uploadViaSingleRequest(
  runtime: UploadRuntime,
  target: {
    filename: string;
    path: string;
  },
  file: UppyFile<UploadMeta, UploadBody>,
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
    headers: authHeaders(runtime.accessToken),
    onProgress,
    url: "/api/v1/files/upload",
  });
}

async function uploadViaMultipart(
  runtime: UploadRuntime,
  target: {
    filename: string;
    path: string;
  },
  file: UppyFile<UploadMeta, UploadBody>,
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
      headers: authHeaders(runtime.accessToken),
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
        accessToken: runtime.accessToken,
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
        headers: authHeaders(runtime.accessToken),
      });
    if (completeError) {
      throw new Error(getErrorMessage(completeError, completeResponse.status));
    }

    return completeData as UploadBody | undefined;
  } catch (error) {
    await abortMultipartUpload(runtime, initiatePayload);
    throw error;
  }
}

async function abortMultipartUpload(
  runtime: UploadRuntime,
  multipart: GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadInitiateResponse,
) {
  await postApiV1FilesUploadMultipartAbort({
    client: apiClient,
    body: {
      key: multipart.key,
      upload_id: multipart.upload_id,
    },
    headers: authHeaders(runtime.accessToken),
  });
}

async function previewBatchConflicts(
  runtime: UploadRuntime,
  mode: UploadMode,
  files: UppyFile<UploadMeta, UploadBody>[],
): Promise<GithubComAbhishekPenDriveBackendInternalApiDtoDuplicatePreviewResponse | null> {
  if (files.length === 0) {
    return null;
  }

  const relativePaths = files.map((file) =>
    mode === "folder" ? file.meta.relativePath || file.name : file.name,
  );

  const { data, error, response } = await postApiV1FilesDuplicatesPreview({
    client: apiClient,
    body: {
      path: runtime.currentPath,
      relative_paths: relativePaths,
    },
    headers: authHeaders(runtime.accessToken),
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
      headers: authHeaders(runtime.accessToken),
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
      file.meta.relativePath ? "folder" : "file",
      runtime.currentPath,
      file,
    );
    const responseBody = isMultipartEligible(file)
      ? await uploadViaMultipart(runtime, target, file, conflictPolicy, (bytesUploaded, bytesTotal) =>
          updateFileProgress(uppy, file.id, bytesUploaded, bytesTotal),
        )
      : await uploadViaSingleRequest(runtime, target, file, conflictPolicy, (bytesUploaded, bytesTotal) =>
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
  accessToken,
  chunk,
  filename,
  key,
  partNumber,
  uploadId,
  onProgress,
}: {
  accessToken: string;
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
      headers: authHeaders(accessToken),
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
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Conflicts found in this upload</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-2 max-h-64 overflow-y-auto">
          {conflictDialog.items
            .filter((item) => item.conflict)
            .map((item, index) => (
              <div
                className="rounded border p-3 text-sm space-y-1"
                key={item.requested_path || item.rename_path || `conflict-${index}`}
              >
                <p><span className="font-medium">Existing: </span>{item.existing_path || item.requested_path || "unknown"}</p>
                <p><span className="font-medium">Rename target: </span>{item.rename_path || "not available"}</p>
              </div>
            ))}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onCancel}>Cancel upload</Button>
          <Button variant="outline" onClick={() => onSelect(DuplicateConflictPolicy.DUPLICATE_CONFLICT_POLICY_RENAME)}>
            Create renamed copies
          </Button>
          <Button onClick={() => onSelect(DuplicateConflictPolicy.DUPLICATE_CONFLICT_POLICY_REPLACE)}>
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

function formatBytes(bytes: number) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }

  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes;
  let unitIndex = -1;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  return `${value.toFixed(value >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}
