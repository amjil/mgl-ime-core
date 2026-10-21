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
- `USER_FULL_JSON` (user candidate overlay, default `user_full.json` in data dir)
- `USER_NEXT_JSON` (user next-word overlay, default `user_next.json` in data dir)

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

### Add user word (overlay)

- `POST /api/user_words`

Request:

```json
{"word":"ᠠ","nextWords":["ᠰᠡᠪᠡ","ᠬᠡ"]}
```

`word` is required. `nextWords` is an optional string list; when non-empty, those entries are prepended to the next-word list for `word` in `user_next.json` (at most 64 entries).

The server writes `user_full.json` under two lookup keys derived from `word`:

- the full word text
- the abbreviated key (leading vowel kept when the word starts with a vowel; other vowels removed; MVS/FVS/NNBSP and other non-letters stripped)

Response:

```json
{"status":"ok"}
```

Query APIs merge base dictionaries with these user overlay files (user entries appear first).

## Data Source

The server loads both files during startup:

- `full.json` or `full.json.gz` for candidates
- `next.json` or `next.json.gz` for next words
- `user_full.json` and `user_next.json` for user-added words (created on first write)

Gzip is detected by file magic, so a `.gz` suffix is not required.

Each file is a JSON object whose values are lists. List items may be:

- a string: `"word"`
- a ranked tuple: `["word", 0.12]` or `["word", "12"]`

Only the word text is kept. Existing rank order is preserved.
