# CoolCassette

[English](README.md) | [中文](README_CN.md)

把你的音乐库变成 Sony NW 系列 Walkman 上 [Wampy](https://github.com/thedannicraft/wampy) 播放器的定制磁带皮肤。

CoolCassette 读取专辑封面，用 AI 生成独特的磁带视觉效果（外壳 + 标签 + 动态卷轴），然后直接部署到设备上。

---

## 功能

- 扫描音乐库，从文件标签或封面图自动提取专辑封面
- 用 AI 生成完整的 800×480 磁带皮肤
- 为每盘磁带创建动态卷轴动画（40 帧旋转循环）
- 直接部署到 Walkman 的 `wampy/skins/` 目录
- 桌面应用：专辑浏览、音频试听、一键生成皮肤
- 命令行工具：批量处理和自动化

---

## 快速开始

### 桌面应用 (macOS)

1. 从 [Releases](../../releases) 下载 `CoolCassette-<version>-macos-arm64.tar.gz`
2. 解压，将 `CoolCassette.app` 拖入 Applications
3. 打开应用，在设置中配置音乐目录
4. 点选任意专辑 → 生成预览 → 发布

### 命令行

```bash
# 编译安装
git clone https://github.com/coolcassette/coolcassette
cd coolcassette && go build -o coolcassette .

# 配置 API 密钥
echo '{"api_key":"sk-or-...","provider":"openrouter"}' > ~/.coolcassette.json
# 或: export OPENROUTER_API_KEY=sk-or-...

# 预览单个专辑
coolcassette preview ~/Music/AlbumName

# 批量生成并部署
coolcassette generate --music-dir ~/Music --wampy-dir /Volumes/WALKMAN/wampy
```

---

## 依赖

| 工具 | 用途 | 安装方式 |
|------|------|----------|
| ImageMagick 7 | 封面缩放、磁带合成、卷轴图集 | `brew install imagemagick` |

AI 服务账号（任选其一）：
- [OpenRouter](https://openrouter.ai)（推荐，默认）
- [Google AI](https://ai.google.dev)（Gemini）

> `etc1tool`（Android Platform Tools）已包含在 Release 下载包中。CLI 用户可从 [Android Developer](https://developer.android.com/tools/releases/platform-tools) 下载，放在二进制文件旁的 `platform-tools/` 目录中即可。

---

## 命令

### `preview` — 生成并预览单个专辑

```bash
coolcassette preview ~/Music/Artist/Album
```

在专辑目录下生成 `tape.png` 和 `reel.png`。`generate` 命令会复用缓存，跳过 API 调用。

### `generate` — 批量生成并部署

```bash
coolcassette generate --music-dir ~/Music --wampy-dir /Volumes/WALKMAN/wampy
```

扫描所有专辑，为未处理的专辑生成皮肤并部署到设备。已有 `cassette.txt` 的专辑会被跳过。加 `--force` 强制重新生成。

### `share` — 导出便携皮肤

```bash
coolcassette share --music-dir ~/Music --output-dir ./share
```

生成皮肤到本地目录，附带独立的 `preview.html` 预览文件，不需要连接设备。

### `server` — 启动 API 服务

```bash
coolcassette server --listen 127.0.0.1:7350
```

启动 HTTP API 服务器，供桌面应用或自定义集成使用。

### `uninstall` — 移除已部署的皮肤

```bash
coolcassette uninstall --music-dir ~/Music --wampy-dir /Volumes/WALKMAN/wampy --dry-run
```

删除所有皮肤并重置 `cassette.txt`。加 `--dry-run` 预览操作。

---

## 配置

### `~/.coolcassette.json`

```json
{
  "api_key": "sk-or-...",
  "provider": "openrouter"
}
```

优先级：**命令行参数 > 环境变量 > 配置文件 > 默认值**

| 参数 | 环境变量 | 默认值 | 说明 |
|------|----------|--------|------|
| `--api-key` | `OPENROUTER_API_KEY`, `API_KEY` | 配置文件 | AI 服务 API 密钥 |
| `--provider` | `PROVIDER` | `openrouter` | `openrouter` 或 `google` |
| `--music-dir` | — | — | 音乐库路径（可重复） |
| `--wampy-dir` | — | — | 设备上的 wampy 目录 |
| `--shell` | — | `random` | 外壳模板：`chf`、`bhf`、`random` |
| `--force` | — | false | 强制重新生成已有皮肤 |
| `--verbose` | — | false | 详细输出 |

---

## 支持的格式

MP3、FLAC、WAV、M4A、M4B、AAC、MP4

封面优先级：专辑目录中的 `cover.jpg/png` → 嵌入式标签 → AI 生成
