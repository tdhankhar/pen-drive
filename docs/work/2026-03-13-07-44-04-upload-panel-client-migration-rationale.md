# Upload Panel Client Migration Rationale

Keep `frontend/src/components/upload-panel.tsx` aligned with the generated API client as much as possible.

Brief rationale:

- reduces contract drift between frontend upload requests and backend OpenAPI
- makes backend field changes such as `conflict_policy` show up in TypeScript immediately
- centralizes request/response typing instead of hand-parsing multiple `fetch` responses
- keeps the component focused on upload orchestration and UI state instead of raw endpoint details
- makes `npm run api:generate`, lint, and build a meaningful integration check

Pragmatic boundary:

- keep handwritten orchestration only where the UI genuinely needs it, such as multipart chunk sequencing
- prefer generated client calls for single upload, multipart initiate, part, complete, and abort requests
