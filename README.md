# Badge Editing Plugin for Apache Answer

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go)](https://go.dev/)
[![Apache Answer](https://img.shields.io/badge/Apache%20Answer-%3E%3D1.7.0-D22128)](https://answer.apache.org/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0)

Language: English | [简体中文](README.zh-CN.md)

Admin-facing badge management plugin for [Apache Answer](https://answer.apache.org/), including badge CRUD, manual award/revoke, and transaction-safe write operations.

## Table of Contents

- [Why This Plugin](#why-this-plugin)
- [Feature Highlights](#feature-highlights)
- [Compatibility](#compatibility)
- [Installation](#installation)
- [Quick Verification](#quick-verification)
- [API Overview](#api-overview)
- [API Details](#api-details)
- [Behavior and Consistency Guarantees](#behavior-and-consistency-guarantees)
- [Internationalization](#internationalization)
- [Development Notes](#development-notes)
- [Known Limitations](#known-limitations)
- [License](#license)

## Why This Plugin

Apache Answer supports badges out of the box, but operational teams often need:

- A direct admin API for full badge lifecycle management.
- Manual assignment/revocation for support, migration, or moderation workflows.
- Concurrency-safe behavior when multiple admins operate simultaneously.

This plugin targets those needs while staying compatible with Answer core ID rules and DB schema expectations.

## Feature Highlights

- Badge CRUD:
  - Create badges
  - Update badges
  - Soft-delete badges
  - List active badges
- Badge group discovery:
  - Query all badge groups for form/dropdown selection
- Manual awarding:
  - Award badge by `username`
  - Revoke badge awards by `username`
- Data safety:
  - Transaction-wrapped writes
  - Row-level locking on award path (`FOR UPDATE`)
  - Atomic `award_count` increment/decrement
- Core-compatible ID generation:
  - Same format as Apache Answer object IDs

## Compatibility

| Item | Version |
|------|---------|
| Apache Answer | `>= 1.7.0` |
| Go | `>= 1.23` |
| Module path | `github.com/wchiways/badge-editing` |

## Installation

1. Add plugin dependency:

```bash
go get github.com/wchiways/badge-editing@latest
```

2. Import plugin in your Answer build:

```go
import _ "github.com/wchiways/badge-editing"
```

3. Rebuild Answer:

```bash
go mod tidy
go build -o answer
```

## Quick Verification

After service startup, call any admin API endpoint with an administrator token/cookie.

Example:

```bash
curl -X GET 'http://127.0.0.1:9080/answer/admin/api/badge-editing/badge-groups' \
  -H 'Authorization: Bearer <admin-token>'
```

If configuration is correct, response is HTTP `200` with `{"data":[...]}`.

## API Overview

All endpoints are registered under the admin router and require administrator authentication.

Base path:

```text
/answer/admin/api/badge-editing
```

### Badge Management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/badges` | List all non-deleted badges |
| `POST` | `/badges` | Create badge |
| `PUT` | `/badges` | Update badge |
| `DELETE` | `/badges/:id` | Soft-delete badge |

### Badge Groups

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/badge-groups` | List badge groups |

### Manual Award / Revoke

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/badges/award` | Award badge to a user |
| `DELETE` | `/badges/award` | Revoke badge awards from a user |

## API Details

### 1. Create Badge

`POST /answer/admin/api/badge-editing/badges`

Request body:

```json
{
  "name": "Top Contributor",
  "icon": "trophy",
  "description": "Awarded to top contributors",
  "level": 3,
  "badge_group_id": 1,
  "single": 1
}
```

Field rules:

| Field | Type | Required | Notes |
|------|------|----------|-------|
| `name` | `string` | Yes | Badge name |
| `icon` | `string` | Yes | Icon identifier |
| `description` | `string` | No | Description text |
| `level` | `int` | Yes | `1` bronze, `2` silver, `3` gold |
| `badge_group_id` | `int64` | Yes | Must exist in `badge_group` table |
| `single` | `int` | Yes | `1` single-award per user, `2` multiple awards |

Success response: HTTP `200`, `{"data": {...badge...}}`

### 2. Update Badge

`PUT /answer/admin/api/badge-editing/badges`

Request body:

```json
{
  "id": "10090000000000001",
  "name": "Top Contributor",
  "icon": "trophy",
  "description": "Updated description",
  "level": 3,
  "badge_group_id": 1,
  "single": 1,
  "status": 1
}
```

Extra fields:

| Field | Type | Required | Notes |
|------|------|----------|-------|
| `id` | `string` | Yes | Target badge ID |
| `status` | `int` | No | `1` active, `11` inactive |

Possible responses:

- `404` when badge does not exist or is already deleted.
- `400` when target badge group does not exist.

### 3. Delete Badge (Soft Delete)

`DELETE /answer/admin/api/badge-editing/badges/:id`

Behavior:

- Badge `status` is updated to deleted (`10`).
- Related `badge_award` rows are marked with `is_badge_deleted=1`.
- Both operations are in one transaction.

Success response:

```json
{
  "message": "badge deleted successfully"
}
```

### 4. List Badge Groups

`GET /answer/admin/api/badge-editing/badge-groups`

Success response:

```json
{
  "data": [
    { "id": "1", "name": "Gold" },
    { "id": "2", "name": "Silver" }
  ]
}
```

### 5. Award Badge

`POST /answer/admin/api/badge-editing/badges/award`

Request body:

```json
{
  "badge_id": "10090000000000001",
  "username": "john_doe"
}
```

Behavior:

- User lookup requires `status = 1`.
- Badge row is locked (`FOR UPDATE`) inside transaction.
- For `single=1`, duplicate active awards return conflict.
- `award_count` is incremented atomically.

Common responses:

- `200` awarded successfully
- `404` user not found / badge not found or inactive
- `409` badge already awarded to this user

### 6. Revoke Badge Award

`DELETE /answer/admin/api/badge-editing/badges/award`

Request body:

```json
{
  "badge_id": "10090000000000001",
  "username": "john_doe"
}
```

Behavior:

- Deletes matching rows from `badge_award`.
- Decrements `award_count` by deleted row count in the same transaction.

Success response:

```json
{
  "message": "badge award revoked successfully",
  "revoked_count": 1
}
```

## Behavior and Consistency Guarantees

- Transaction boundaries:
  - `CreateBadge`, `DeleteBadge`, `AwardBadge`, `RevokeBadgeAward` are transactional.
- Concurrency:
  - Award path locks badge row to avoid race conditions in duplicate checks and counts.
- ID strategy:
  - `genUniqueID` follows Answer core format:
    - `"1"` + 3-digit object type + 13-digit numeric ID
  - Examples:
    - Badge object type: `9`
    - Badge award object type: `10`

## Internationalization

Included locales:

- `en_US`
- `zh_CN`

Translation files are stored in [`i18n/en_US.yaml`](i18n/en_US.yaml) and [`i18n/zh_CN.yaml`](i18n/zh_CN.yaml).

## Development Notes

- Core implementation: [`badge_editing.go`](badge_editing.go)
- Plugin metadata: [`info.yaml`](info.yaml)
- Module definition: [`go.mod`](go.mod)

Useful commands:

```bash
go test ./...
go vet ./...
```

## Known Limitations

- This plugin assumes Apache Answer core tables already exist (`badge`, `badge_group`, `badge_award`, `user`, `uniqid`).
- No public unauthenticated/user-auth routes are exposed; only admin routes are implemented.
- Revoke operation deletes award rows physically (not soft-delete).

## License

Licensed under the [Apache License 2.0](http://www.apache.org/licenses/LICENSE-2.0).

Last Reviewed: 2026-03-04
