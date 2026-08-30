# Go Backend

## Run

```bash
cd backend
go run ./cmd/server
```

The server also resolves `backend/full.json` (or `.gz`) when started from the repository root.

Optional env:

- `PORT` (default `3003`)
- `DATA_DIR` (directory that contains the JSON files)
- `FULL_JSON` (explicit candidates file path)
- `NEXT_JSON` (explicit next-word file path)

## APIs

### Health check

- `GET /healthz`

Response:

```json
{"status":"ok"}
```

### Query candidates

- `POST /api/next_word/candidates`

Request:

```json
{"word":"mong"}
```

Response:

```json
{"candidates":["...","..."]}
```

### Query next words list

- `POST /api/next_word/list`

Request:

```json
{"word":"mong"}
```

Response:

```json
{"nextWords":["...","..."]}
```

Request body is limited to 4KB. `word` is required and at most 256 characters.

## Data Source

The server loads both files during startup:

- `full.json` or `full.json.gz` for candidates
- `next.json` or `next.json.gz` for next words

Gzip is detected by file magic, so a `.gz` suffix is not required.

Each file is a JSON object whose values are lists. List items may be:

- a string: `"word"`
- a ranked tuple: `["word", 0.12]` or `["word", "12"]`

Only the word text is kept. Existing rank order is preserved.
