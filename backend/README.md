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
{"candidates":[["ᠬᠠᠨᠢ", 1000000000000000], ["...", 2]]}
```

Each entry is `[word, order]`. Dictionary order is preserved. Words added through `POST /api/user_words` use order `1e15`, then `1e15 - 1`, and so on, so a client that sorts by descending order shows the feedback first. The next query sees that feedback without restarting the server.

### Query next words list

- `POST /api/next_word/list`

Request:

```json
{"word":"mong"}
```

Response:

```json
{"nextWords":[["...",1000000000000000],["...",57223]]}
```

Same `[word, order]` shape as candidates. User feedback is ordered at `1e15` and comes first when the list is sorted by descending order.

Request body is limited to 4KB. `word` is required and at most 256 characters.

### Add user word (overlay)

- `POST /api/user_words`

Request:

```json
{"word":"ᠠ","nextWords":["ᠰᠡᠪᠡ","ᠬᠡ"]}
```

`word` is required. `nextWords` is an optional string list; when non-empty, those entries are prepended to the next-word list for `word` in `user_next.json` (at most 64 entries). The write updates the in-memory overlay and clears the query cache, so the following candidates or next-word request includes it. Feedback words are returned with order `1e15` and sort ahead of dictionary scores.

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

The numeric order is kept and returned. A bare string has order `0`. User overlay files store word text only; query responses assign those words order `1e15`.
