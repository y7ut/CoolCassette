# CoolCassette

[English](README.md) | [中文](README_CN.md)

AI 驱动的磁带皮肤生成器，专为 Sony NW 系列 Walkman 设备上的 [Wampy](https://github.com/unknown321/wampy) 音乐播放器插件设计。

扫描你的音乐库，提取专辑封面，使用 AI 生成定制的磁带视觉效果，并直接部署到设备上 — 包括动态卷轴动画。

---

## 它能做什么

- **自动提取专辑封面**：从音频文件标签（MP3、FLAC、WAV、M4A）或专辑目录中的封面图片提取封面
- **磁带贴纸和动画**：基于专辑封面，使用 AI 生成独特的磁带外观，包括标签艺术和相配的磁带外壳，并支持旋转动画
- **一键部署到设备**：将生成的皮肤和动画自动压缩并部署到 Walkman 设备的 wampy skins 目录
- **自动关联专辑**：在每个专辑目录生成配置文件，让 Wampy 自动为每张专辑匹配对应的磁带皮肤

---

## 系统要求

- Go 1.21+
- [ImageMagick 7](https://imagemagick.org)（`magick` 命令）
- Android Platform Tools `etc1tool` — 用于 PKM 压缩  
  下载：[Mac](https://dl.google.com/android/repository/platform-tools-latest-darwin.zip) · [Windows](https://dl.google.com/android/repository/platform-tools-latest-windows.zip) · [Linux](https://dl.google.com/android/repository/platform-tools-latest-linux.zip)  
  将 `etc1tool` 放在二进制文件旁边的 `platform-tools/` 中或 `PATH` 中
- [OpenRouter](https://openrouter.ai) API 密钥（或 OpenAI）

---

## 安装

```bash
git clone https://github.com/coolcassette/coolcassette
cd coolcassette
go build -o coolcassette .
```

设置你的 API 密钥：

```bash
export OPENROUTER_API_KEY=sk-or-...
# 或
export OPENAI_API_KEY=sk-...
```

---

## 命令

### `preview`

为单个专辑目录生成磁带预览。将 `tape.png` 和 `reel.png` 与音乐文件一起保存，以便在完整生成运行之前进行检查。

```bash
coolcassette preview ~/Music/Nujabes/Modal\ Soul \
  --api-key $OPENROUTER_API_KEY
```

缓存的 `tape.png` 会被 `generate` 命令重用，跳过 API 调用。

---

### `generate`

扫描音乐目录并为所有未处理的专辑生成 + 部署皮肤。

```bash
coolcassette generate \
  --music-dir /Volumes/WALKMAN/MUSIC \
  --wampy-dir /Volumes/WALKMAN/wampy \
  --api-key $OPENROUTER_API_KEY
```

每个专辑获得：
- `wampy/skins/cassette/tape/<slug>_tape/` — 磁带皮肤（PKM + 配置）
- `wampy/skins/cassette/reel/<slug>_reel/` — 卷轴图集（PKM + atlas.txt + 配置）
- `<album-dir>/cassette.txt` — Wampy 的皮肤分配

已有有效 `cassette.txt` 的专辑将被跳过（已处理）。使用 `--force` 重新生成。

**封面图像优先级：**
1. 专辑目录中的 `cover.{jpg,jpeg,png,webp}`（调整为 400×400）
2. 音频文件标签中的嵌入封面
3. API 调用以从头生成

---

### `share`

将皮肤构建为可移植目录，而无需部署到设备。生成自包含的 `preview.html`，其中嵌入了磁带动画（无需外部文件）。

```bash
coolcassette share \
  --music-dir ~/Music/Nujabes \
  --api-key $OPENROUTER_API_KEY \
  --output-dir ./share
```

输出结构：

```
share/
  <Artist>/
    <Album>/
      tape/<slug>_tape/
        tape.pkm
        config.txt
      reel/<slug>_reel/
        atlas.pkm
        atlas.txt
        config.txt
      cassette.txt
      preview.html     ← 自包含的磁带动画，在浏览器中打开
```

---

### `uninstall`

删除所有已部署的皮肤并将专辑目录恢复到其原始状态。

```bash
coolcassette uninstall \
  --music-dir /Volumes/WALKMAN/MUSIC \
  --wampy-dir /Volumes/WALKMAN/wampy
```

读取每个 `cassette.txt`，从 wampy 中删除相应的磁带/卷轴目录，删除缓存的 `tape.png`/`reel.png`，并从专辑目录中删除 `cassette.txt`。

使用 `--dry-run` 预览将要删除的内容。

---

## 全局标志

| 标志 | 默认值 | 描述 |
|------|--------|------|
| `--music-dir` | — | 音乐根目录路径 |
| `--wampy-dir` | — | 设备上的 wampy 目录路径 |
| `--api-key` | env | API 密钥（`OPENROUTER_API_KEY` 或 `OPENAI_API_KEY`） |
| `--provider` | `openrouter` | `openrouter` 或 `openai` |
| `--shell` | `random` | 外壳模板：`chf`、`bhf` 或 `random` |
| `--reel` | `other` | 如果每专辑卷轴失败，则使用回退卷轴名称 |
| `--force` | false | 重新处理已有 `cassette.txt` 的专辑 |
| `--dry-run` | false | 打印计划而不写入任何文件 |
| `--verbose` | false | 详细输出 |

---

## 皮肤命名

为避免磁带和卷轴目录之间的冲突，皮肤使用音频标签元数据命名：

- Slug 格式：`<artist>_<album>`（已清理，小写）
- 磁带目录：`<slug>_tape`
- 卷轴目录：`<slug>_reel`

如果缺少标签，则使用专辑目录路径作为回退。

---

## 卷轴动画

卷轴精灵直接从磁带图像生成：

- 模板区域：磁带上位置 (180, 161) 的 440×110 px
- 提取两个圆：左中心 (57, 56)，右中心 (383, 56)，半径 42
- 40 帧 × 9° 旋转 = 完整 360°
- 帧延迟：55ms（Wampy 默认值）
- 图集布局：4 列 × 10 行 → 1760×1100 px PNG → ETC1 PKM

---

## 支持的音频格式

MP3、FLAC、WAV、M4A、M4B、AAC、MP4
