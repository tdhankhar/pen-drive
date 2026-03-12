import Uppy, { type UploadResult, type UppyFile } from "@uppy/core";
import {
  Dropzone,
  FilesList,
  UploadButton,
  UppyContextProvider,
  useUppyState,
} from "@uppy/react";
import {
  type ChangeEvent,
  type MutableRefObject,
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

import {
  postApiV1FilesDuplicatesPreview,
  postApiV1FilesUpload,
  postApiV1FilesUploadMultipartAbort,
  postApiV1FilesUploadMultipartComplete,
  postApiV1FilesUploadMultipartInitiate,
  postApiV1FilesUploadMultipartPart,
  GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy as DuplicateConflictPolicy,
} from "../lib/api/generated";
import type {
  GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy,
  GithubComAbhishekPenDriveBackendInternalApiDtoDuplicatePreviewItem,
  GithubComAbhishekPenDriveBackendInternalApiDtoDuplicatePreviewResponse,
  GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadInitiateResponse,
  GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadPartResponse,
  PostApiV1FilesUploadData,
} from "../lib/api/generated";
import { apiClient } from "../lib/api/http";

const MULTIPART_THRESHOLD_BYTES = 5 * 1024 * 1024;

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
              Drag a folder onto the target or pick one from disk. Each item lands
              under <code>{currentPath || "root"}</code>, and files above 5 MB use
              multipart upload.
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
            description="Drop a folder here or pick one using the button above."
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
  const fileCount = useUppyState(uppy, (state) => Object.keys(state.files).length);

  return (
    <UppyContextProvider uppy={uppy}>
      <div className="flex flex-col gap-2 px-6 pb-6">
        <Dropzone height="180px" note={description} width="100%" />
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">{fileCount} queued</p>
          <UploadButton />
        </div>
        <FilesList />
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

    for (const fileID of fileIDs) {
      const file = uppy.getFile(fileID);
      if (!file) {
        continue;
      }

      try {
        const target = resolveUploadTarget(mode, runtime.currentPath, file);
        let responseBody: UploadBody | undefined;

        if (isMultipartEligible(file)) {
          responseBody = await uploadViaMultipart(
            runtime,
            target,
            file,
            conflictPolicy ?? DuplicateConflictPolicy.DUPLICATE_CONFLICT_POLICY_REJECT,
          );
        } else {
          responseBody = await uploadViaSingleRequest(
            runtime,
            target,
            file,
            conflictPolicy ?? DuplicateConflictPolicy.DUPLICATE_CONFLICT_POLICY_REJECT,
          );
        }

        uppy.emit("upload-success", file, {
          body: responseBody,
          status: 201,
          uploadURL: undefined,
        });
      } catch (error) {
        uppy.emit(
          "upload-error",
          file,
          error instanceof Error ? error : new Error("upload failed"),
        );
      }
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

  const { data, error, response } = await postApiV1FilesUpload({
    client: apiClient,
    body,
    ...authHeaders(runtime.accessToken),
  });
  if (error) throw new Error(getErrorMessage(error, response.status));

  return data as UploadBody | undefined;
}

async function uploadViaMultipart(
  runtime: UploadRuntime,
  target: {
    filename: string;
    path: string;
  },
  file: UppyFile<UploadMeta, UploadBody>,
  conflictPolicy: GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy,
): Promise<UploadBody | undefined> {
  if (!(file.data instanceof Blob) || typeof file.size !== "number") {
    throw new Error("file data is unavailable");
  }

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
    for (
      let offset = 0;
      offset < file.data.size;
      offset += initiatePayload.part_size
    ) {
      const chunk = file.data.slice(offset, offset + initiatePayload.part_size);
      const { data: partData, error: partError, response: partResponse } =
        await postApiV1FilesUploadMultipartPart({
          client: apiClient,
          body: {
            key: initiatePayload.key,
            part: new File([chunk], `${target.filename}.part-${partNumber}`, {
              type: "application/octet-stream",
            }),
            part_number: partNumber,
            upload_id: initiatePayload.upload_id,
          },
          headers: authHeaders(runtime.accessToken),
        });
      if (partError) {
        throw new Error(getErrorMessage(partError, partResponse.status));
      }
      const partPayload = requireMultipartPart(partData);

      uploadedParts.push({
        etag: partPayload.etag,
        part_number: partPayload.part_number,
      });
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
    ...authHeaders(runtime.accessToken),
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
    ...authHeaders(runtime.accessToken),
  });
  if (error) throw new Error(getErrorMessage(error, response.status));

  return data ?? null;
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
