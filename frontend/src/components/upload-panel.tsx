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
} from "../lib/api/generated";

// TODO(Task 2): replace with PostApiV1FilesUploadData['body'] from generated types
type PostApiV1FilesUploadBody = {
  file: Blob | File;
  path?: string;
  filename?: string;
  conflict_policy?: string;
};

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
      <section className="upload-grid">
        <UploadCard
          description="Drag files onto the target or browse from this device. Files above 5 MB switch to multipart upload automatically."
          error={fileError}
          message={fileMessage}
          title="Quick file upload"
          uppy={fileUppy}
        />
        <div className="upload-card">
          <div className="upload-copy">
            <p className="panel-label">Folder upload</p>
            <h2>Preserve nested paths</h2>
            <p>
              Drag a folder onto the target or pick one from disk. Each item lands
              under <code>{currentPath || "root"}</code>, and files above 5 MB use
              multipart upload.
            </p>
          </div>
          <div className="upload-toolbar">
            <button
              className="secondary-button"
              onClick={() => folderInputRef.current?.click()}
              type="button"
            >
              Pick folder
            </button>
            <button
              className="secondary-button"
              onClick={() => folderUppy.cancelAll()}
              type="button"
            >
              Clear queue
            </button>
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
          {folderMessage ? <p className="upload-feedback">{folderMessage}</p> : null}
          {folderError ? (
            <p className="upload-feedback upload-feedback-error">{folderError}</p>
          ) : null}
        </div>
      </section>
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
    <div className="upload-card">
      <div className="upload-copy">
        <p className="panel-label">Upload</p>
        <h2>{title}</h2>
        <p>{description}</p>
      </div>
      <div className="upload-toolbar">
        <button
          className="secondary-button"
          onClick={() => uppy.cancelAll()}
          type="button"
        >
          Clear queue
        </button>
      </div>
      <UppySurface
        description="Drag files here or click the surface to browse."
        uppy={uppy}
      />
      {message ? <p className="upload-feedback">{message}</p> : null}
      {error ? <p className="upload-feedback upload-feedback-error">{error}</p> : null}
    </div>
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
      <div className="uppy-shell">
        <Dropzone height="180px" note={description} width="100%" />
        <div className="uppy-footer">
          <p className="uppy-queue">{fileCount} queued</p>
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
            conflictPolicy ?? DuplicateConflictPolicy.DuplicateConflictPolicyReject,
          );
        } else {
          responseBody = await uploadViaSingleRequest(
            runtime,
            target,
            file,
            conflictPolicy ?? DuplicateConflictPolicy.DuplicateConflictPolicyReject,
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

  const body: PostApiV1FilesUploadBody = {
    file: new File([file.data], file.name, { type: file.type }),
  };
  if (target.path) {
    body.path = target.path;
  }
  if (target.filename !== file.name) {
    body.filename = target.filename;
  }
  body.conflict_policy = conflictPolicy;

  const response = await postApiV1FilesUpload(body, authorizedRequest(runtime));
  if (response.status !== 201) {
    throw new Error(getErrorMessage(response.data, response.status));
  }

  return response.data as UploadBody | undefined;
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

  const initiateResponse = await postApiV1FilesUploadMultipartInitiate(
    {
      content_type: file.type,
      conflict_policy: conflictPolicy,
      filename: target.filename,
      path: target.path,
      size: file.size,
    },
    authorizedRequest(runtime),
  );
  if (initiateResponse.status !== 201) {
    throw new Error(getErrorMessage(initiateResponse.data, initiateResponse.status));
  }
  const initiatePayload = requireMultipartSession(initiateResponse.data);

  const uploadedParts: Array<{ etag: string; part_number: number }> = [];

  try {
    let partNumber = 1;
    for (
      let offset = 0;
      offset < file.data.size;
      offset += initiatePayload.part_size
    ) {
      const chunk = file.data.slice(offset, offset + initiatePayload.part_size);
      const partResponse = await postApiV1FilesUploadMultipartPart(
        {
          key: initiatePayload.key,
          part: new File([chunk], `${target.filename}.part-${partNumber}`, {
            type: "application/octet-stream",
          }),
          part_number: partNumber,
          upload_id: initiatePayload.upload_id,
        },
        authorizedRequest(runtime),
      );
      if (partResponse.status !== 200) {
        throw new Error(getErrorMessage(partResponse.data, partResponse.status));
      }
      const partPayload = requireMultipartPart(partResponse.data);

      uploadedParts.push({
        etag: partPayload.etag,
        part_number: partPayload.part_number,
      });
      partNumber += 1;
    }

    const completeResponse = await postApiV1FilesUploadMultipartComplete(
      {
        key: initiatePayload.key,
        parts: uploadedParts,
        upload_id: initiatePayload.upload_id,
      },
      authorizedRequest(runtime),
    );
    if (completeResponse.status !== 201) {
      throw new Error(getErrorMessage(completeResponse.data, completeResponse.status));
    }

    return completeResponse.data as UploadBody | undefined;
  } catch (error) {
    await abortMultipartUpload(runtime, initiatePayload);
    throw error;
  }
}

async function abortMultipartUpload(
  runtime: UploadRuntime,
  multipart: GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadInitiateResponse,
) {
  await postApiV1FilesUploadMultipartAbort(
    {
      key: multipart.key,
      upload_id: multipart.upload_id,
    },
    authorizedRequest(runtime),
  );
}

function authorizedRequest(runtime: UploadRuntime): RequestInit {
  return {
    headers: {
      Authorization: `Bearer ${runtime.accessToken}`,
    },
  };
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

  const response = await postApiV1FilesDuplicatesPreview(
    {
      path: runtime.currentPath,
      relative_paths: relativePaths,
    },
    authorizedRequest(runtime),
  );
  if (response.status !== 200) {
    throw new Error(getErrorMessage(response.data, response.status));
  }

  return response.data;
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
  payload: GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadInitiateResponse,
) {
  if (
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
  payload: GithubComAbhishekPenDriveBackendInternalApiDtoMultipartUploadPartResponse,
) {
  if (!payload.etag || typeof payload.part_number !== "number") {
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
    <div className="conflict-dialog-backdrop" role="presentation">
      <div
        aria-labelledby="conflict-dialog-title"
        aria-modal="true"
        className="conflict-dialog"
        role="dialog"
      >
        <div className="conflict-dialog-copy">
          <p className="panel-label">Duplicate preview</p>
          <h2 id="conflict-dialog-title">Conflicts found in this upload</h2>
          <p>
            {conflictDialog.mode === "folder" ? "Folder upload" : "File upload"}{" "}
            would affect {conflictDialog.impactedPaths.length} existing
            {conflictDialog.impactedPaths.length === 1 ? " path" : " paths"}.
          </p>
        </div>
        <div className="conflict-dialog-list">
          {conflictDialog.items
            .filter((item) => item.conflict)
            .map((item, index) => (
              <article
                className="conflict-item"
                key={item.requested_path || item.rename_path || `conflict-${index}`}
              >
                <p>
                  <strong>Existing</strong>
                  <span>{item.existing_path || item.requested_path || "unknown"}</span>
                </p>
                <p>
                  <strong>Rename target</strong>
                  <span>{item.rename_path || "not available"}</span>
                </p>
              </article>
            ))}
        </div>
        <div className="conflict-dialog-actions">
          <button className="secondary-button" onClick={onCancel} type="button">
            Cancel upload
          </button>
          <button
            className="secondary-button"
            onClick={() => onSelect(DuplicateConflictPolicy.DuplicateConflictPolicyRename)}
            type="button"
          >
            Create renamed copies
          </button>
          <button
            className="primary-button"
            onClick={() => onSelect(DuplicateConflictPolicy.DuplicateConflictPolicyReplace)}
            type="button"
          >
            Replace existing files
          </button>
        </div>
      </div>
    </div>
  );
}
