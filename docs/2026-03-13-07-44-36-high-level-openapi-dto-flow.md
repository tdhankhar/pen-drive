# High-Level: Go → React DTO via OpenAPI

## Flow

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Go Backend     │     │  OpenAPI Spec   │     │  React Frontend │
│  DTOs + Router  │────▶│  (yaml/json)    │────▶│  Generated TS   │
└─────────────────┘     └─────────────────┘     └─────────────────┘
        │                       │                       │
   Run `openapi            openapi.yaml           Import generated
   generate` CLI           pushed to repo          types + client
```

## How It Works

1. **Backend defines DTOs** in `backend/internal/api/dto/`
   - Pure Go structs with JSON tags
   - Example: `TokenPair`, `AuthResponse`

2. **Backend router annotated** with OpenAPI comments (`@Summary`, `@Param`, etc.)
   - Gin routes get OpenAPI metadata

3. **CLI generates spec** (`openapi generate`)
   - Parses Go code + annotations
   - Outputs `openapi.yaml`

4. **Frontend runs code generator** (openapi-typescript-codegen)
   - Consumes `openapi.yaml`
   - Outputs `src/lib/api/generated.ts` with:
     - TypeScript interfaces matching Go DTOs
     - Typed HTTP client functions

## Benefits

| Aspect | Benefit |
|--------|---------|
| Type safety | Frontend types match backend DTOs exactly |
| No manual sync | DTO changes auto-propagate |
| Self-documenting | OpenAPI spec is always up-to-date |

## Key Files

| Path | Role |
|------|------|
| `backend/internal/api/dto/*.go` | Go DTO definitions |
| `backend/internal/http/router.go` | Annotated Gin routes |
| `openapi.yaml` | Generated spec |
| `frontend/src/lib/api/generated.ts` | Generated TS types + client |
| `frontend/src/lib/api/http.ts` | Custom fetch wrapper |

## Commands

```bash
# Generate OpenAPI spec (run from backend)
make openapi

# Generate frontend types (run from frontend)
npm run codegen
```
