# Badge Editing Plugin for Apache Answer

A plugin for [Apache Answer](https://answer.apache.org/) that provides administrators with full badge management capabilities, including CRUD operations and manual badge awarding/revoking.

## Features

- Create, update, delete (soft) badges
- List all badges and badge groups
- Manually award badges to users
- Revoke badge awards from users
- Transaction-safe operations with row-level locking
- ID generation compatible with Answer core format

## Requirements

- Apache Answer >= 1.7.0
- Go >= 1.23

## Installation

Add the plugin import to your Answer build:

```go
import _ "github.com/wchiways/badge-editing"
```

Then rebuild Answer:

```bash
go mod tidy
go build -o answer
```

## API Endpoints

All endpoints require **administrator authentication** and are registered under the admin router group.

### Badges

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/answer/admin/api/badge-editing/badges` | List all active badges |
| `POST` | `/answer/admin/api/badge-editing/badges` | Create a new badge |
| `PUT` | `/answer/admin/api/badge-editing/badges` | Update an existing badge |
| `DELETE` | `/answer/admin/api/badge-editing/badges/:id` | Soft delete a badge |

### Badge Groups

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/answer/admin/api/badge-editing/badge-groups` | List all badge groups |

### Badge Awards

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/answer/admin/api/badge-editing/badges/award` | Award a badge to a user |
| `DELETE` | `/answer/admin/api/badge-editing/badges/award` | Revoke a badge from a user |

## API Reference

### Create Badge

```json
POST /answer/admin/api/badge-editing/badges
{
  "name": "Top Contributor",
  "icon": "trophy",
  "description": "Awarded to top contributors",
  "level": 3,
  "badge_group_id": 1,
  "single": 1
}
```

**Fields:**
- `name` (string, required) - Badge name
- `icon` (string, required) - Badge icon identifier
- `description` (string) - Badge description
- `level` (int, required) - Badge level: `1` (Bronze), `2` (Silver), `3` (Gold)
- `badge_group_id` (int64, required) - Badge group ID
- `single` (int, required) - Award type: `1` (single award per user), `2` (multiple awards allowed)

### Update Badge

```json
PUT /answer/admin/api/badge-editing/badges
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

**Additional fields:**
- `id` (string, required) - Badge ID
- `status` (int) - Badge status: `1` (Active), `11` (Inactive)

### Delete Badge

```
DELETE /answer/admin/api/badge-editing/badges/:id
```

Soft deletes the badge and marks all related awards as deleted within a single transaction.

### Award Badge

```json
POST /answer/admin/api/badge-editing/badges/award
{
  "badge_id": "10090000000000001",
  "username": "john_doe"
}
```

Awards a badge to the specified user. For single-award badges, duplicate awards are prevented. The operation uses row-level locking (`SELECT ... FOR UPDATE`) to prevent race conditions.

### Revoke Badge Award

```json
DELETE /answer/admin/api/badge-editing/badges/award
{
  "badge_id": "10090000000000001",
  "username": "john_doe"
}
```

Revokes all awards of the specified badge from the user and decrements the award count atomically.

## Internationalization

The plugin supports i18n with the following languages:
- English (`en_US`)
- Simplified Chinese (`zh_CN`)

## Technical Notes

- **ID Format**: Generated IDs follow Answer core format: `1` + 3-digit object type + 13-digit auto-increment ID (e.g., `10090000000000042` for badge type 9)
- **Transaction Safety**: All write operations (create, delete, award, revoke) are wrapped in database transactions with automatic rollback on failure
- **Concurrency Control**: Badge awarding uses `FOR UPDATE` row locking to prevent duplicate awards under concurrent requests
- **Database Access**: The plugin implements the `KVStorage` interface to obtain the `*xorm.Engine` from Answer's plugin system

## License

Licensed under the [Apache License 2.0](http://www.apache.org/licenses/LICENSE-2.0).
