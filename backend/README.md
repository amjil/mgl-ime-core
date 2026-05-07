# Go Backend

## Run

```bash
cd backend
go run ./cmd/server
```

Optional env:

- `PORT` (default `3003`)

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
{"word":"ᠮᠣᠩ"}
```

Response:

```json
{"candidates":["...","..."]}
```

### Query next words list

- `POST /api/next_word/list`

Request:

```json
{"word":"ᠮᠣᠩ"}
```

Response:

```json
{"nextWords":["...","..."]}
```

## Data Source

The server loads both files during startup:

- `backend/full.json` for candidates
- `backend/next.json` for next words

Both files are expected to be `map[string][]string`.
