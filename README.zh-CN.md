# Apache Answer Badge Editing 插件

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go)](https://go.dev/)
[![Apache Answer](https://img.shields.io/badge/Apache%20Answer-%3E%3D1.7.0-D22128)](https://answer.apache.org/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0)

语言: 简体中文 | [English](README.md)

面向管理员的 Apache Answer 徽章管理插件，支持徽章 CRUD、手动授予/撤销，并提供事务级写入安全保障。

## 目录

- [为什么需要这个插件](#为什么需要这个插件)
- [核心特性](#核心特性)
- [兼容性](#兼容性)
- [安装方式](#安装方式)
- [快速验证](#快速验证)
- [API 总览](#api-总览)
- [API 详细说明](#api-详细说明)
- [一致性与并发保障](#一致性与并发保障)
- [国际化](#国际化)
- [开发说明](#开发说明)
- [已知限制](#已知限制)
- [许可证](#许可证)

## 为什么需要这个插件

Apache Answer 原生支持徽章能力，但在实际运维或运营场景中，通常还需要：

- 管理后台可直接调用的完整徽章生命周期 API。
- 支持人工补发/回收徽章，满足客服、迁移、风控等场景。
- 多管理员并发操作下，依然保证数据一致性和计数正确。

本插件围绕这些需求设计，并保持与 Answer Core 的 ID 规则和数据库表结构兼容。

## 核心特性

- 徽章 CRUD:
  - 创建徽章
  - 更新徽章
  - 软删除徽章
  - 查询未删除徽章列表
- 徽章组查询:
  - 获取全部徽章组，便于前端下拉选择
- 手动授予/撤销:
  - 按 `username` 授予徽章
  - 按 `username` 撤销徽章
- 数据安全:
  - 写操作全部事务化
  - 授予流程对徽章行加锁（`FOR UPDATE`）
  - `award_count` 原子增减
- ID 规则兼容:
  - 与 Apache Answer Core 保持同格式 ID 生成策略

## 兼容性

| 项目 | 版本 |
|------|------|
| Apache Answer | `>= 1.7.0` |
| Go | `>= 1.23` |
| 模块路径 | `github.com/wchiways/badge-editing` |

## 安装方式

1. 添加插件依赖：

```bash
go get github.com/wchiways/badge-editing@latest
```

2. 在 Answer 构建入口中引入插件：

```go
import _ "github.com/wchiways/badge-editing"
```

3. 重新构建 Answer：

```bash
go mod tidy
go build -o answer
```

## 快速验证

服务启动后，使用管理员 Token/Cookie 调用任一管理 API。

示例：

```bash
curl -X GET 'http://127.0.0.1:9080/answer/admin/api/badge-editing/badge-groups' \
  -H 'Authorization: Bearer <admin-token>'
```

若配置正常，返回 HTTP `200`，结构类似 `{"data":[...]}`。

## API 总览

所有接口都注册在管理员路由下，必须具备管理员权限。

基础前缀：

```text
/answer/admin/api/badge-editing
```

### 徽章管理

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/badges` | 查询所有未删除徽章 |
| `POST` | `/badges` | 创建徽章 |
| `PUT` | `/badges` | 更新徽章 |
| `DELETE` | `/badges/:id` | 软删除徽章 |

### 徽章组

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/badge-groups` | 查询徽章组 |

### 手动授予/撤销

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/badges/award` | 给用户授予徽章 |
| `DELETE` | `/badges/award` | 撤销用户的徽章授予 |

## API 详细说明

### 1. 创建徽章

`POST /answer/admin/api/badge-editing/badges`

请求体：

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

字段约束：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | `string` | 是 | 徽章名称 |
| `icon` | `string` | 是 | 图标标识 |
| `description` | `string` | 否 | 描述文本 |
| `level` | `int` | 是 | `1` 铜牌，`2` 银牌，`3` 金牌 |
| `badge_group_id` | `int64` | 是 | 必须是已存在的徽章组 ID |
| `single` | `int` | 是 | `1` 每用户仅一次，`2` 允许多次授予 |

成功响应：HTTP `200`，`{"data": {...badge...}}`

### 2. 更新徽章

`PUT /answer/admin/api/badge-editing/badges`

请求体：

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

额外字段：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | `string` | 是 | 待更新徽章 ID |
| `status` | `int` | 否 | `1` 启用，`11` 停用 |

常见响应：

- `404` 徽章不存在或已删除
- `400` 目标徽章组不存在

### 3. 删除徽章（软删除）

`DELETE /answer/admin/api/badge-editing/badges/:id`

行为说明：

- 将徽章 `status` 更新为已删除（`10`）。
- 将关联 `badge_award` 记录标记为 `is_badge_deleted=1`。
- 两步操作在同一个事务中完成。

成功响应：

```json
{
  "message": "badge deleted successfully"
}
```

### 4. 查询徽章组

`GET /answer/admin/api/badge-editing/badge-groups`

成功响应：

```json
{
  "data": [
    { "id": "1", "name": "Gold" },
    { "id": "2", "name": "Silver" }
  ]
}
```

### 5. 授予徽章

`POST /answer/admin/api/badge-editing/badges/award`

请求体：

```json
{
  "badge_id": "10090000000000001",
  "username": "john_doe"
}
```

行为说明：

- 用户查询要求 `status = 1`。
- 事务内对徽章行加锁（`FOR UPDATE`）。
- 当 `single=1` 时，重复授予会返回冲突。
- `award_count` 通过原子更新递增。

常见响应：

- `200` 授予成功
- `404` 用户不存在 / 徽章不存在或不可用
- `409` 该用户已获得该徽章

### 6. 撤销徽章授予

`DELETE /answer/admin/api/badge-editing/badges/award`

请求体：

```json
{
  "badge_id": "10090000000000001",
  "username": "john_doe"
}
```

行为说明：

- 删除 `badge_award` 中匹配记录。
- 在同一事务内按删除条数递减 `award_count`。

成功响应：

```json
{
  "message": "badge award revoked successfully",
  "revoked_count": 1
}
```

## 一致性与并发保障

- 事务边界:
  - `CreateBadge`
  - `DeleteBadge`
  - `AwardBadge`
  - `RevokeBadgeAward`
- 并发控制:
  - 授予流程通过锁定徽章行，避免并发下重复授予和计数偏差。
- ID 策略:
  - `genUniqueID` 遵循 Answer Core 规则：
    - `"1"` + 3 位对象类型 + 13 位数字 ID
  - 示例：
    - 徽章对象类型：`9`
    - 徽章授予对象类型：`10`

## 国际化

当前包含语言：

- `en_US`
- `zh_CN`

翻译文件位于 [i18n/en_US.yaml](i18n/en_US.yaml) 与 [i18n/zh_CN.yaml](i18n/zh_CN.yaml)。

## 开发说明

- 核心实现：[badge_editing.go](badge_editing.go)
- 插件元信息：[info.yaml](info.yaml)
- 模块定义：[go.mod](go.mod)

常用命令：

```bash
go test ./...
go vet ./...
```

## 已知限制

- 依赖 Apache Answer Core 已存在以下表：`badge`、`badge_group`、`badge_award`、`user`、`uniqid`。
- 仅实现管理员路由，未提供匿名路由和普通登录用户路由。
- 撤销操作为物理删除 `badge_award` 记录（非软删除）。

## 许可证

遵循 [Apache License 2.0](http://www.apache.org/licenses/LICENSE-2.0)。

最后审阅日期：2026-03-04
