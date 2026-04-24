# CoolCassette 服务端接口文档

本文档面向前端联调与服务端维护，描述 `coolcassette server` 当前提供的 HTTP API、索引一致性机制、参数定义和示例。

## 1. 服务启动

### 使用场景

- 本地启动一个 HTTP 服务，扫描音乐目录，建立 SQLite 索引，供前端分页浏览专辑、查看详情、触发预览与发布。

### 启动命令

```bash
coolcassette server \
  --music-dir /path/to/MUSIC \
  --wampy-dir /path/to/wampy \
  --listen 127.0.0.1:7350
```

### 说明

- `--music-dir` 必填，指向音乐库根目录。
- `--wampy-dir` 必填，指向 Wampy 目录。即使本地暂时没有真实 Wampy，也可以先给一个空目录，列表和索引功能仍然可用。
- 服务启动时会同步扫描整个音乐树，构建 SQLite 索引后才开始监听。

## 2. 核心概念

### 2.1 专辑状态

当前专辑状态字段 `status` 有 3 种取值：

- `built`
  说明专辑目录下存在 `cassette.txt`，并且引用的 `tape` / `reel` 资源在 `wampy` 目录中存在且文件完整。
- `preview_ready`
  说明专辑还没有完成发布，但专辑目录中已经存在本地预览缓存 `tape.png`。
- `not_built`
  说明既没有完整发布，也没有本地预览缓存。

### 2.2 索引版本与哈希

服务内部维护一份“当前生效的索引快照”，由两部分标识：

- `index_version`
  表示当前活动索引的版本号。每次完成一次新的索引构建并切换成功，都可能变化。
- `index_hash`
  表示当前活动索引的内容摘要。只有索引内容变化时才会变化。

这两个字段用于滚动分页一致性控制。

### 2.3 响应头

以下索引相关接口会在响应头中返回：

- `X-CoolCassette-Index-Version`
- `X-CoolCassette-Index-Hash`

建议前端在列表页保存这两个值，并在后续分页请求中带回服务端。

### 2.4 一致性规则

列表接口 `GET /api/albums` 支持客户端通过请求头带回当前页面使用的索引快照：

- `X-CoolCassette-Index-Version`
- `X-CoolCassette-Index-Hash`

服务端的处理规则如下：

1. 如果请求里的 `index_version` 与服务端当前版本一致：
   说明仍然是同一份索引，正常返回 `200`。

2. 如果请求里的 `index_version` 过期，但 `index_hash` 与当前索引哈希相同：
   说明索引快照实例变了，但内容没变。正常返回 `200`，并在响应头返回新的 `version/hash`。

3. 如果请求里的 `index_version` 过期，且 `index_hash` 也不同：
   说明内容发生了变化，旧分页游标不再安全。返回 `409 Conflict`，前端应清空列表并从第一页重新拉取。

### 2.5 当前缓存与索引目录

- 本地缓存根目录：
  `os.UserCacheDir()/coolcassette/.cccache/`
- SQLite 索引数据库目录：
  `os.UserCacheDir()/coolcassette/.cccache/index/`
- 封面缓存与已发布 PKM 解码后的 PNG 缓存：
  也位于上述缓存根目录下，以隐藏文件方式保存。

## 3. 通用返回约定

### 成功返回

- 列表、详情、状态查询通常返回 `200 OK`
- 异步触发重扫接口返回 `202 Accepted`

### 常见错误

- `404 Not Found`
  资源不存在，例如专辑 ID 不存在、请求的图片文件不存在。
- `409 Conflict`
  当前请求依赖的索引快照已经过期且内容发生变化，前端需要回到第一页重新请求。
- `500 Internal Server Error`
  服务器内部错误，例如扫描、读取文件、生成图片失败。

### `409` 响应示例

```json
{
  "error": "index content changed",
  "index_version": "20260424T063342.716925000Z",
  "index_hash": "560caceddfd69d3c7f9775f51cd513bce7fbc89aacc8f55fcd104c3ffc76a06b",
  "reload_required": true
}
```

## 4. 接口列表

### 4.1 健康检查

#### `GET /api/health`

#### 使用场景

- 前端启动时探活
- 本地调试时确认 server 是否已启动

#### 请求参数

无

#### 请求示例

```bash
curl http://127.0.0.1:7350/api/health
```

#### 响应示例

```json
{
  "ok": true
}
```

## 5. Library 相关接口

### 5.1 获取当前索引状态

#### `GET /api/library/status`

#### 使用场景

- 前端进入应用时获取当前索引版本与哈希
- 显示扫描状态、专辑总数、后台重建进度
- 判断是否正在执行异步重扫

#### 请求参数

无

#### 请求示例

```bash
curl -i http://127.0.0.1:7350/api/library/status
```

#### 响应头示例

```http
X-CoolCassette-Index-Version: 20260424T063342.716925000Z
X-CoolCassette-Index-Hash: 560caceddfd69d3c7f9775f51cd513bce7fbc89aacc8f55fcd104c3ffc76a06b
```

#### 响应 JSON 示例

```json
{
  "index_version": "20260424T063342.716925000Z",
  "index_hash": "560caceddfd69d3c7f9775f51cd513bce7fbc89aacc8f55fcd104c3ffc76a06b",
  "album_count": 38,
  "scanning": false,
  "scan_id": "20260424T063342.716925000Z",
  "scan_started_at": "2026-04-24T06:33:42.716945Z",
  "scan_finished_at": "2026-04-24T06:33:46.186615Z",
  "scanned_albums": 38,
  "total_albums": 38
}
```

#### 字段说明

- `index_version`
  当前活动索引版本。
- `index_hash`
  当前活动索引内容哈希。
- `album_count`
  当前索引中可见的专辑总数。
- `scanning`
  是否正在执行后台重扫。
- `scan_id`
  当前或最近一次扫描任务 ID。
- `scan_started_at`
  扫描开始时间。
- `scan_finished_at`
  扫描结束时间。若仍在扫描，则可能为空。
- `scanned_albums`
  已写入索引的专辑数量。
- `total_albums`
  本轮扫描预计处理的专辑总数。
- `scan_error`
  若扫描失败，这里会返回错误信息。

### 5.2 异步重建索引go

#### `POST /api/library/reload`

#### 使用场景

- 用户点击“重新扫描音乐库”
- 音乐目录新增、删除或修改后，需要后台构建新索引
- 不希望重扫阻塞当前查询服务

#### 请求参数

无

#### 请求示例

```bash
curl -X POST http://127.0.0.1:7350/api/library/reload
```

#### 响应 JSON 示例

```json
{
  "accepted": true,
  "scan_id": "20260424T145500.123456000Z",
  "index_version": "20260424T063342.716925000Z",
  "index_hash": "560caceddfd69d3c7f9775f51cd513bce7fbc89aacc8f55fcd104c3ffc76a06b",
  "scanning": true
}
```

#### 字段说明

- `accepted`
  是否接受了本次重扫请求。
- `scan_id`
  新后台扫描任务的 ID。
- `index_version`
  当前仍在服务中的旧索引版本。
- `index_hash`
  当前仍在服务中的旧索引哈希。
- `scanning`
  是否已进入后台扫描状态。

#### 补充说明

- 重扫期间，旧索引仍可继续提供列表和详情查询。
- 新索引构建完成后，会原子切换为新的活动索引。

## 6. Album 列表接口

### 6.1 获取专辑列表

#### `GET /api/albums`

#### 使用场景

- 首页专辑列表
- 无限滚动 / 滚动分页加载
- 根据艺术家、专辑名称、创建时间、修改时间排序

#### 请求参数

| 参数名 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `limit` | number | 否 | 单页返回数量，默认 `50`，最大 `200` |
| `sort_by` | string | 否 | 排序字段，支持 `album`、`artist`、`created_at`、`modified_at`，默认 `album` |
| `order` | string | 否 | 排序方向，支持 `asc`、`desc`，默认 `asc` |
| `cursor` | string | 否 | 上一页返回的游标，用于拉取下一页 |

#### 请求头

| Header | 必填 | 说明 |
| --- | --- | --- |
| `X-CoolCassette-Index-Version` | 否 | 当前前端持有的索引版本 |
| `X-CoolCassette-Index-Hash` | 否 | 当前前端持有的索引哈希 |

#### 第一页请求示例

```bash
curl -i "http://127.0.0.1:7350/api/albums?limit=3&sort_by=artist&order=asc"
```

#### 下一页请求示例

```bash
curl -i \
  -H "X-CoolCassette-Index-Version: 20260424T063342.716925000Z" \
  -H "X-CoolCassette-Index-Hash: 560caceddfd69d3c7f9775f51cd513bce7fbc89aacc8f55fcd104c3ffc76a06b" \
  "http://127.0.0.1:7350/api/albums?limit=3&sort_by=artist&order=asc&cursor=opaque-cursor"
```

#### 响应 JSON 示例

```json
{
  "items": [
    {
      "id": "d5f5e1a1f74b6266471d8147d53bef5d0f6088ee",
      "dir": "/Users/xieyichu/Music/Download/Blur - Blur - 1997   Flac",
      "name": "Download/Blur - Blur - 1997   Flac",
      "slug": "blur_blur",
      "artist": "Blur",
      "album": "Blur",
      "track_count": 14,
      "status": "not_built",
      "has_cover": true,
      "cassette_ref_valid": false,
      "cover_url": "/api/albums/d5f5e1a1f74b6266471d8147d53bef5d0f6088ee/assets/cover.png",
      "created_at": "2025-06-28T03:17:40Z",
      "modified_at": "2026-04-19T04:38:53Z"
    }
  ],
  "next_cursor": "eyJzb3J0X2J5IjoiYXJ0aXN0IiwiLi4uIjoiLi4uIn0",
  "has_more": true,
  "index_version": "20260424T063342.716925000Z",
  "index_hash": "560caceddfd69d3c7f9775f51cd513bce7fbc89aacc8f55fcd104c3ffc76a06b"
}
```

#### 响应字段说明

- `items`
  当前页专辑列表。
- `next_cursor`
  下一页游标。若没有更多数据，则可能为空。
- `has_more`
  是否还有下一页。
- `index_version`
  当前返回数据所基于的索引版本。
- `index_hash`
  当前返回数据所基于的索引哈希。

#### `items[]` 字段说明

- `id`
  专辑唯一 ID。
- `dir`
  专辑目录绝对路径。
- `name`
  扫描出的显示名称。
- `slug`
  生成磁带资源时使用的 slug。
- `artist`
  艺术家名。
- `album`
  专辑名。
- `track_count`
  音轨数量。
- `status`
  专辑状态。
- `has_cover`
  是否检测到封面。
- `cassette`
  若存在 `cassette.txt`，则返回其 `tape` / `reel` 引用。
- `cassette_ref_valid`
  `cassette.txt` 引用的资源是否在 Wampy 中有效。
- `cover_url`
  封面资源接口地址。
- `created_at`
  专辑目录创建时间。
- `modified_at`
  专辑目录最后修改时间。

## 7. Album 详情接口

### 7.1 获取单张专辑详情

#### `GET /api/albums/:id`

#### 使用场景

- 用户点击列表中的某张专辑后进入详情页
- 想显示专辑目录中的所有音频文件
- 想显示已发布资源配置、卷轴图集信息

#### 路径参数

| 参数名 | 说明 |
| --- | --- |
| `id` | 专辑唯一 ID |

#### 请求示例

```bash
curl -i http://127.0.0.1:7350/api/albums/d5f5e1a1f74b6266471d8147d53bef5d0f6088ee
```

#### 响应 JSON 示例

```json
{
  "id": "d5f5e1a1f74b6266471d8147d53bef5d0f6088ee",
  "dir": "/Users/xieyichu/Music/Download/Blur - Blur - 1997   Flac",
  "name": "Download/Blur - Blur - 1997   Flac",
  "slug": "blur_blur",
  "artist": "Blur",
  "album": "Blur",
  "track_count": 14,
  "status": "not_built",
  "has_cover": true,
  "cassette_ref_valid": false,
  "cover_url": "/api/albums/d5f5e1a1f74b6266471d8147d53bef5d0f6088ee/assets/cover.png",
  "created_at": "2025-06-28T03:17:40Z",
  "modified_at": "2026-04-19T04:38:53Z",
  "music_files": [
    {
      "name": "01 - Beetlebum.flac",
      "path": "/Users/xieyichu/Music/Download/Blur - Blur - 1997   Flac/01 - Beetlebum.flac"
    }
  ],
  "index_version": "20260424T063342.716925000Z",
  "index_hash": "560caceddfd69d3c7f9775f51cd513bce7fbc89aacc8f55fcd104c3ffc76a06b"
}
```

#### 详情补充说明

- 如果专辑已发布并且 `cassette_ref_valid = true`，响应中还会补充：
  - `published_tape_png_url`
  - `published_reel_png_url`
  - `tape_config`
  - `reel_config`
  - `reel_atlas_frames`

## 8. Album 写操作接口

### 8.1 生成或刷新本地预览

#### `POST /api/albums/:id/preview`

#### 使用场景

- 用户想先生成本地 `tape.png` / `reel.png` 预览，不立即发布到 Wampy
- 需要强制重新生成专辑的预览图

#### 路径参数

| 参数名 | 说明 |
| --- | --- |
| `id` | 专辑唯一 ID |

#### 请求体

```json
{
  "force": false
}
```

#### 参数说明

- `force`
  - `false`：尽量复用已有缓存
  - `true`：强制删除本地预览缓存后重新生成

#### 请求示例

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"force":true}' \
  http://127.0.0.1:7350/api/albums/d5f5e1a1f74b6266471d8147d53bef5d0f6088ee/preview
```

#### 响应示例

返回该专辑最新详情 JSON，结构与 `GET /api/albums/:id` 基本一致。

### 8.2 生成并发布到 Wampy

#### `POST /api/albums/:id/publish`

#### 使用场景

- 用户确认专辑预览可用后，正式生成磁带资源并写入 Wampy
- 想强制重新生成并重新部署该专辑

#### 路径参数

| 参数名 | 说明 |
| --- | --- |
| `id` | 专辑唯一 ID |

#### 请求体

```json
{
  "force": false
}
```

#### 请求示例

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"force":true}' \
  http://127.0.0.1:7350/api/albums/d5f5e1a1f74b6266471d8147d53bef5d0f6088ee/publish
```

#### 响应说明

- 返回该专辑最新详情 JSON。
- 如果本地 `wampy` 目录为空或资源不存在，则 `status` 大概率仍然不是 `built`。

## 9. 图片资源接口

### 9.1 获取专辑封面缓存

#### `GET /api/albums/:id/assets/cover.png`

#### 使用场景

- 列表页展示封面
- 详情页展示专辑封面

#### 说明

- 服务端会读取专辑封面，并在本地缓存目录中生成隐藏缓存文件。
- 实际返回的图片可能是 JPEG，即使路径名写的是 `cover.png`，前端应以 `Content-Type` 为准。

#### 请求示例

```bash
curl -O http://127.0.0.1:7350/api/albums/d5f5e1a1f74b6266471d8147d53bef5d0f6088ee/assets/cover.png
```

### 9.2 获取本地预览图

#### `GET /api/albums/:id/assets/tape.png`

#### 使用场景

- 详情页展示当前专辑目录中的本地预览磁带图

#### 说明

- 如果该专辑尚未执行过 `preview` 或 `publish`，则可能返回 `404`。

#### `GET /api/albums/:id/assets/reel.png`

#### 使用场景

- 详情页展示当前专辑目录中的本地预览卷轴图

#### 说明

- 如果该专辑尚未执行过 `preview` 或 `publish`，则可能返回 `404`。

### 9.3 获取已发布资源的可预览 PNG

#### `GET /api/albums/:id/published/tape.png`

#### 使用场景

- 详情页展示已经部署到 Wampy 的 `tape.pkm` 对应的可预览图片

#### 说明

- 服务端会把 `wampy` 中的 `tape.pkm` 解码为 PNG，并缓存到本地隐藏目录后返回。
- 若该专辑尚未发布，或 Wampy 中资源不存在，则返回 `404`。

#### `GET /api/albums/:id/published/reel.png`

#### 使用场景

- 详情页展示已经部署到 Wampy 的 `atlas.pkm` 对应的可预览图集

#### 说明

- 服务端会把 `atlas.pkm` 解码为 PNG，并缓存到本地隐藏目录后返回。
- 若该专辑尚未发布，或 Wampy 中资源不存在，则返回 `404`。

## 10. 前端接入建议

### 10.1 首页加载流程

1. 先请求 `GET /api/library/status`
2. 记录返回的：
   - `index_version`
   - `index_hash`
3. 请求 `GET /api/albums?limit=...`
4. 保存返回的：
   - `next_cursor`
   - `index_version`
   - `index_hash`

### 10.2 无限滚动流程

1. 下一页请求时继续带上：
   - `cursor`
   - `X-CoolCassette-Index-Version`
   - `X-CoolCassette-Index-Hash`
2. 若返回 `200`：
   - 追加 `items`
   - 更新 `next_cursor`
   - 更新响应头中的新版 `version/hash`
3. 若返回 `409`：
   - 清空当前列表
   - 以服务端返回的新 `version/hash` 从第一页重新拉取

### 10.3 详情页加载流程

1. 请求 `GET /api/albums/:id`
2. 封面使用 `cover_url`
3. 若需要本地预览，尝试加载：
   - `/api/albums/:id/assets/tape.png`
   - `/api/albums/:id/assets/reel.png`
4. 若需要展示已发布资源，尝试加载：
   - `/api/albums/:id/published/tape.png`
   - `/api/albums/:id/published/reel.png`

## 11. 目前已知说明

- 当 `wampy` 目录本地为空时：
  - `cassette_ref_valid` 通常为 `false`
  - `status` 大多为 `not_built` 或 `preview_ready`
- 图片资源接口当前以 `GET` 为主，若使用 `HEAD`，不保证一定返回与 `GET` 完全一致的行为。
- 封面资源路径名虽然是 `cover.png`，但真实返回类型可能是 `image/jpeg`，应以响应头为准。
