---
name: openlist-api
description: 'Use when calling OpenList APIs, reading fox.oplist.org API docs, generating curl or code examples, troubleshooting OpenList authentication, listing files, reading file metadata, creating folders, creating shares, or mapping a user goal to the correct OpenList endpoint. Keywords: OpenList, openlist 接口, fox.oplist.org, API, JWT, fs/list, fs/get, fs/mkdir, share/create.'
argument-hint: 'Describe the OpenList task, base URL, auth state, target language, and the endpoint or business goal.'
user-invocable: true
---

# OpenList API Calling

## Purpose

Use this skill when the user wants to call an OpenList API, find the right OpenList endpoint, turn the OpenList docs into executable requests, or debug a failing OpenList request.

This skill is for practical API work:
- identify the correct endpoint from the user's goal
- inspect the official docs under `https://fox.oplist.org` via the local reference files [API docs quick reference](./references/api-docs.md) and [Schema quick reference](./references/schema-docs.md)
- generate working `curl`, JavaScript/TypeScript, Python, or other client examples
- explain request and response structures
- troubleshoot authentication, permissions, path, and payload issues

## Mandatory Rules

1. Treat `https://fox.oplist.org` as the canonical documentation host.
2. Skip `https://fox.oplist.org/llms.txt` during normal execution. Use the local reference files in `./references/` first, because they already contain the canonical `https://fox.oplist.org/...` links.
3. Do not assume the OpenList token uses the `Bearer ` prefix. The official docs state that the `Authorization` header should contain the JWT token directly.
4. Prefer the official endpoint doc for the exact request body and response shape before writing code.
5. When the user gives only a business goal, map it to the narrowest endpoint instead of proposing broad, unnecessary API sequences.

## Reference Files

- [API docs quick reference](./references/api-docs.md): endpoint doc index derived from `llms.txt`, with all doc links rewritten to `https://fox.oplist.org`
- [Schema quick reference](./references/schema-docs.md): schema doc index derived from `llms.txt`, with all doc links rewritten to `https://fox.oplist.org`

## When To Use

- The user says they want to call an OpenList API.
- The user provides `fox.oplist.org` docs or mentions OpenList/OpenList API/OpenList 接口.
- The user wants request samples for login, file listing, file metadata, folder creation, sharing, storage admin, or user admin.
- The user wants to translate OpenList docs into code.
- The user has an OpenList error such as `401`, `403`, `404`, invalid path, or wrong payload.

## Workflow

### 1. Pin down the task

Extract these inputs from the user request:
- target OpenList base URL, such as `http://host:5244`
- whether authentication already exists
- the operation they want to perform
- the desired output form: explanation, `curl`, JS/TS, Python, Go, or direct code integration

If one of these is missing and is required to produce a correct answer, ask only for the missing piece.

### 2. Find the correct endpoint

Start from the business goal, then map to the endpoint family:
- Authentication: `/api/auth/login`, logout, 2FA, SSO, WebAuthn
- User: current user profile and SSH keys
- Admin: users, storages, drivers, settings, metas, search index
- File System: `/api/fs/list`, `/api/fs/get`, `/api/fs/tree`, `/api/fs/mkdir`, rename, move, copy, remove, upload, archive operations
- Public: settings, offline download tools, archive extensions
- Sharing: `/api/share/create`, list/update/delete/enable/disable shares

Use [API docs quick reference](./references/api-docs.md) to choose the endpoint document. Only fall back to direct browsing if the local references do not cover the needed doc.

### 3. Read the exact endpoint doc

For the chosen endpoint, extract and restate:
- HTTP method
- path
- whether auth is required
- header requirements
- request body fields, including required fields
- success response shape
- common error responses relevant to the request

Keep this step concrete. Do not hand-wave request fields if the endpoint doc defines them.

### 4. Apply the auth rules correctly

Use this auth decision logic:
- If the endpoint is public, no token is required.
- If the endpoint requires auth and the user has no token yet, use `POST /api/auth/login` first.
- Login body normally includes `username` and `password`, and may include `otp_code` when 2FA is enabled.
- After login, read `data.token` from the response.
- For authenticated endpoints, send `Authorization: <token>`.
- Do not prepend `Bearer ` unless the user has verified their server expects a different format.

### 5. Generate the smallest useful request

Prefer minimal, runnable examples.

Good output patterns:
- one `curl` request with the exact JSON body
- a short JS/TS `fetch` example
- a short Python `requests` example
- a focused helper function when the user is integrating into an app

Avoid generating a full SDK unless the user explicitly asks for one.

### 6. Validate against the response contract

Before finalizing, check:
- required request fields are present
- path strings look valid for OpenList, for example `/`, `/folder`, `/document.pdf`
- the auth requirement matches the endpoint
- the response handling reads the documented fields, such as `code`, `message`, and `data`
- pagination parameters are included only where the endpoint supports them

### 7. Troubleshoot if the request fails

Use this error triage:
- `401`: token missing, expired, invalid, or sent with the wrong header format
- `403`: insufficient permissions, wrong password for protected path, or admin-only action attempted by non-admin user
- `404`: path not found or wrong endpoint path
- `400`: malformed body, missing required field, invalid path, or operation conflict such as creating an existing directory

When debugging, compare the failing request to the exact endpoint doc and correct the smallest mismatch.

## High-Value Endpoint Shortlist

These are common starting points:

- Login: `POST /api/auth/login`
  - doc: `https://fox.oplist.org/364155678e0.md`
  - body: `username`, `password`, optional `otp_code`
  - success: `data.token`

- List directory: `POST /api/fs/list`
  - doc: `https://fox.oplist.org/364155732e0.md`
  - auth: required
  - body: `path`, optional `password`, `refresh`, `page`, `per_page`

- Get file or directory info: `POST /api/fs/get`
  - doc: `https://fox.oplist.org/364155733e0.md`
  - auth: required
  - body: `path`, optional `password`

- Create directory: `POST /api/fs/mkdir`
  - doc: `https://fox.oplist.org/364155737e0.md`
  - auth: required
  - body: `path`

- Create share: `POST /api/share/create`
  - doc: `https://fox.oplist.org/364155757e0.md`
  - auth: required
  - body: `paths`, optional `password`, `expiration`

## Ready-Made Examples

### Login with curl

```bash
curl -X POST "$OPENLIST_BASE_URL/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "your-password"
  }'
```

### List a directory with curl

```bash
curl -X POST "$OPENLIST_BASE_URL/api/fs/list" \
  -H "Content-Type: application/json" \
  -H "Authorization: $OPENLIST_TOKEN" \
  -d '{
    "path": "/",
    "page": 1,
    "per_page": 30,
    "refresh": false
  }'
```

### JavaScript fetch example

```js
async function listDirectory(baseUrl, token, path = "/") {
  const response = await fetch(`${baseUrl}/api/fs/list`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: token,
    },
    body: JSON.stringify({
      path,
      page: 1,
      per_page: 30,
      refresh: false,
    }),
  });

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }

  const payload = await response.json();
  if (payload.code !== 200) {
    throw new Error(payload.message || "OpenList request failed");
  }

  return payload.data;
}
```

## Completion Criteria

The skill has been applied well only if the answer:
- uses the correct endpoint for the user's goal
- cites or reflects the exact official doc semantics
- uses `https://fox.oplist.org` for documentation references
- handles auth correctly, especially the raw `Authorization` token format
- provides a runnable request or precise implementation guidance
- mentions the most likely failure mode when the request is sensitive to permissions, path, or payload shape

## Example Prompts

- `Use openlist-api: 帮我写一个 curl，请登录 OpenList 然后列出 /movies 目录。`
- `Use openlist-api: 用 TypeScript 封装 OpenList 的 /api/fs/get 和 /api/fs/list。`
- `Use openlist-api: OpenList 返回 401，帮我检查登录和 Authorization 头。`
- `Use openlist-api: 我想给 /docs 和 /images 创建分享链接，给我 Python 示例。`