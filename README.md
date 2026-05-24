# S3 File Viewer

A web app for browsing S3 buckets and querying text files with SQL. Built with Go, HTMX, the AWS SDK, and DuckDB.

## Features

- Browse S3 buckets with folder-style prefix navigation
- View `.csv` and `.txt` files only
- Inspect column names and types for tabular files
- Run SQL queries against a file using DuckDB (via the `data` view)
- Raw text preview fallback for unstructured files

## Prerequisites

- Go 1.23+
- AWS credentials configured via the [default credential chain](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-globals.html) (environment variables, `~/.aws/credentials`, or IAM role)
- Network access on first run (DuckDB downloads the `httpfs` extension)

## Configuration

| Variable | Default | Description |
|---|---|---|
| `AWS_REGION` | *(required)* | AWS region for S3. Falls back to `AWS_DEFAULT_REGION`. |
| `PORT` | `8080` | HTTP listen port |
| `MAX_QUERY_ROWS` | `1000` | Max rows returned per query (LIMIT appended if missing) |
| `QUERY_TIMEOUT_SEC` | `30` | Query timeout in seconds |
| `BUCKET_ALLOWLIST` | *(empty)* | Comma-separated bucket names to allow (empty = all) |
| `RAW_PREVIEW_BYTES` | `262144` | Max bytes shown in raw text preview |

## Run locally

```bash
export AWS_REGION=us-east-1
go run ./cmd/server
```

Open [http://localhost:8080](http://localhost:8080), enter a bucket name, and browse.

## Usage

1. **Home** — Enter a bucket name and optional prefix, then click Browse.
2. **Browse** — Click folders to navigate; click `.csv` or `.txt` files to open them. Other file types are listed but not viewable.
3. **File view** — For tabular files:
   - **Columns** pane loads automatically with schema info
   - **SQL Query** pane runs queries against the `data` view (e.g. `SELECT * FROM data LIMIT 100`)
   - **Results** pane shows query output

For unstructured text files, a raw preview is shown instead of the SQL interface.

## SQL safety

- Only `SELECT` queries are allowed
- Multi-statement queries (semicolons) are rejected
- Destructive keywords (`DROP`, `INSERT`, `CREATE`, etc.) are blocked
- Results are capped at `MAX_QUERY_ROWS`

## Project structure

```
cmd/server/          HTTP server entrypoint
internal/config/     Environment configuration
internal/s3/         AWS S3 listing and object reads
internal/duckdb/     DuckDB pool, schema, and query execution
internal/files/      Text file detection and reader selection
internal/handlers/   HTTP handlers, templates, and static assets
```

