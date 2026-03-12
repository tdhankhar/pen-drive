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

import { apiBaseUrl } from "../lib/api/http";

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

type MultipartInitiateResponse = {
  key: string;
  name: string;
  part_size: number;
  upload_id: string;
};

type MultipartPartResponse = {
  etag: string;
  part_number: number;
};

export function UploadPanel({
  accessToken,
  currentPath,
  onUploaded,
}: UploadPanelProps) {
  const runtimeRef = useRef<UploadRuntime>({ accessToken, currentPath });
  const folderInputRef = useRef<HTMLInputElement | null>(null);
  const [fileMessage, setFileMessage] = useState<string | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [folderMessage, setFolderMessage] = useState<string | null>(null);
  const [folderError, setFolderError] = useState<string | null>(null);
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

  useEffect(() => {
    runtimeRef.current = { accessToken, currentPath };
  }, [accessToken, currentPath]);

  useEffect(() => {
    configureUppy({
      mode: "file",
      setError: setFileError,
      uppy: fileUppy,
      runtimeRef,
    });
    configureUppy({
      mode: "folder",
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
  runtimeRef,
  setError,
  uppy,
}: {
  mode: UploadMode;
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

    for (const fileID of fileIDs) {
      const file = uppy.getFile(fileID);
      if (!file) {
        continue;
      }

      try {
        const runtime = runtimeRef.current;
        const target = resolveUploadTarget(mode, runtime.currentPath, file);
        let responseBody: UploadBody | undefined;

        if (isMultipartEligible(file)) {
          responseBody = await uploadViaMultipart(runtime, target, file);
        } else {
          responseBody = await uploadViaSingleRequest(runtime, target, file);
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
): Promise<UploadBody | undefined> {
  if (!(file.data instanceof Blob)) {
    throw new Error("file data is unavailable");
  }

  const formData = new FormData();
  formData.append("file", file.data, file.name);
  if (target.path) {
    formData.append("path", target.path);
  }
  if (target.filename !== file.name) {
    formData.append("filename", target.filename);
  }

  const response = await fetch(`${apiBaseUrl}/api/v1/files/upload`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${runtime.accessToken}`,
    },
    body: formData,
  });

  const payload = await parseApiPayload(response);
  if (!response.ok) {
    throw new Error(getErrorMessage(payload, response.status));
  }

  return payload as UploadBody | undefined;
}

async function uploadViaMultipart(
  runtime: UploadRuntime,
  target: {
    filename: string;
    path: string;
  },
  file: UppyFile<UploadMeta, UploadBody>,
): Promise<UploadBody | undefined> {
  if (!(file.data instanceof Blob) || typeof file.size !== "number") {
    throw new Error("file data is unavailable");
  }

  const initiateResponse = await fetch(
    `${apiBaseUrl}/api/v1/files/upload-multipart/initiate`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${runtime.accessToken}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        content_type: file.type,
        filename: target.filename,
        path: target.path,
        size: file.size,
      }),
    },
  );

  const initiatePayload = await parseApiPayload(initiateResponse);
  if (!initiateResponse.ok || !isMultipartInitiateResponse(initiatePayload)) {
    throw new Error(getErrorMessage(initiatePayload, initiateResponse.status));
  }

  const uploadedParts: Array<{ etag: string; part_number: number }> = [];

  try {
    let partNumber = 1;
    for (
      let offset = 0;
      offset < file.data.size;
      offset += initiatePayload.part_size
    ) {
      const chunk = file.data.slice(offset, offset + initiatePayload.part_size);
      const formData = new FormData();
      formData.append("upload_id", initiatePayload.upload_id);
      formData.append("key", initiatePayload.key);
      formData.append("part_number", String(partNumber));
      formData.append("part", chunk, `${target.filename}.part-${partNumber}`);

      const partResponse = await fetch(`${apiBaseUrl}/api/v1/files/upload-multipart/part`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${runtime.accessToken}`,
        },
        body: formData,
      });

      const partPayload = await parseApiPayload(partResponse);
      if (!partResponse.ok || !isMultipartPartResponse(partPayload)) {
        throw new Error(getErrorMessage(partPayload, partResponse.status));
      }

      uploadedParts.push({
        etag: partPayload.etag,
        part_number: partPayload.part_number,
      });
      partNumber += 1;
    }

    const completeResponse = await fetch(
      `${apiBaseUrl}/api/v1/files/upload-multipart/complete`,
      {
        method: "POST",
        headers: {
          Authorization: `Bearer ${runtime.accessToken}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          key: initiatePayload.key,
          parts: uploadedParts,
          upload_id: initiatePayload.upload_id,
        }),
      },
    );

    const completePayload = await parseApiPayload(completeResponse);
    if (!completeResponse.ok) {
      throw new Error(getErrorMessage(completePayload, completeResponse.status));
    }

    return completePayload as UploadBody | undefined;
  } catch (error) {
    await abortMultipartUpload(runtime, initiatePayload);
    throw error;
  }
}

async function abortMultipartUpload(
  runtime: UploadRuntime,
  multipart: MultipartInitiateResponse,
) {
  await fetch(`${apiBaseUrl}/api/v1/files/upload-multipart/abort`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${runtime.accessToken}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      key: multipart.key,
      upload_id: multipart.upload_id,
    }),
  });
}

async function parseApiPayload(response: Response) {
  const contentType = response.headers.get("Content-Type") || "";
  if (!contentType.includes("application/json")) {
    return undefined;
  }

  return (await response.json()) as unknown;
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

function isMultipartInitiateResponse(
  payload: unknown,
): payload is MultipartInitiateResponse {
  if (!payload || typeof payload !== "object") {
    return false;
  }

  return (
    "key" in payload &&
    typeof payload.key === "string" &&
    "part_size" in payload &&
    typeof payload.part_size === "number" &&
    "upload_id" in payload &&
    typeof payload.upload_id === "string"
  );
}

function isMultipartPartResponse(payload: unknown): payload is MultipartPartResponse {
  if (!payload || typeof payload !== "object") {
    return false;
  }

  return (
    "etag" in payload &&
    typeof payload.etag === "string" &&
    "part_number" in payload &&
    typeof payload.part_number === "number"
  );
}
