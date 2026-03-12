# pen-drive

## Setup & Development

### Quick Start with MinIO (Local Testing)

Start all services (PostgreSQL, MinIO) and run the backend with MinIO:
```bash
make backend-dev-minio
```

This command:
- Starts PostgreSQL and MinIO containers (idempotent)
- Creates the S3 bucket for uploads
- Runs the backend using MinIO credentials from `.env.minio`

### Manual Setup

**Start services:**
```bash
make backend-dev-up
make backend-s3-setup
```

**Run backend with specific S3 target:**
```bash
# MinIO (local)
make backend-run S3_TARGET=minio

# R2 (uses .env.local)
make backend-run
```

**Other commands:**
```bash
make backend-build        # Build backend
make backend-test         # Run tests
make backend-lint         # Format and lint
make backend-dev-down     # Stop services
```