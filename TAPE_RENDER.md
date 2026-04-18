# CoolCassette 磁带贴纸渲染流程

## 最终输出结构

```
wampy/skins/cassette/tape/<album-slug>/
├── tape.pkm      # ETC1 压缩纹理，800×480
└── config.txt    # 文字布局配置
```

每个专辑目录下：
```
Music/<album-dir>/
└── cassette.txt  # tape: <album-slug>\nreel: other
```

---

## 外壳模板

位于 `templates/` 目录，已预先制作好，含透明通道（Alpha）。

| 文件 | 说明 |
|---|---|
| `templates/shell_chf.png` | CHF 款外壳，底部外露 reel 圆轮，复古风 |
| `templates/shell_bhf.png` | BHF 款外壳，全黑简洁风 |

**挖空的标签区域坐标：**
```
x=54, y=42, width=694, height=291
```
即像素范围 `(54,42) → (747,333)`，这是专辑贴纸的绘制区域。

外壳模板制作方法（从原始 blank 图生成）：
```bash
magick tape_chf_blank.png \
  -alpha set \
  \( -clone 0 -alpha extract -fill black -draw "rectangle 54,42 747,333" \) \
  -alpha off -compose CopyOpacity -composite \
  -depth 8 templates/shell_chf.png
```

---

## 渲染流程

### Step 1：准备专辑贴纸 PNG

专辑贴纸尺寸：**694×291 px**（对应标签区域）

内容由专辑封面生成，最终需覆盖整个标签区，具体设计见下方「贴纸设计方案」。

### Step 2：合成完整磁带 PNG（800×480）

```bash
magick -size 800x480 xc:"#000000" \
  \( tape_sticker.png -resize 694x291! \) -geometry +54+42 -composite \
  templates/shell_chf.png -composite \
  -depth 8 tape.png
```

层次（下→上）：
1. 纯黑背景 800×480
2. 专辑贴纸，缩放到 694×291，偏移 `+54+42`
3. 外壳模板（透明区域自动透出贴纸）

### Step 3：ETC1 压缩为 PKM

```bash
./platform-tools/etc1tool tape.png -o tape.pkm
```

依赖：Android platform-tools 中的 `etc1tool`（Mac 版）
下载：https://dl.google.com/android/repository/platform-tools-latest-darwin.zip

### Step 4：生成 config.txt

```yaml
reel: other
artistx: 83.0
artisty: 65.0
artistformat: $ARTIST
titlex: 83.0
titley: 95.0
titleformat: $TITLE
albumx: -1.0
albumy: -1.0
albumformat: $ALBUM
reelx: 134.0
reely: 160.0
titlewidth: 580.0
durationformat: %1$02d:%2$02d
textcolor: #FFFFFF
```

**文字坐标说明：**
- `(0,0)` 是磁带图左上角
- `artisty: 65` / `titley: 95` 将文字置于标签区上半部（标签区 y=42~333）
- 负数坐标表示隐藏该字段（album 默认隐藏）
- `textcolor` 根据贴纸背景亮度自动选 `#FFFFFF` 或 `#000000`
- `reelx/reely: 134/160` 使用文档默认值，reel 动画叠加在机芯窗口区域

### Step 5：生成 cassette.txt（写入音乐目录）

```
tape: <album-slug>
reel: other
```

---

## 贴纸设计方案

### 方案：AI 绘图模型生成（推荐）

效果最佳。向绘图模型 API（如 GPT-Image、Stable Diffusion、Flux 等）发送专辑封面图 + 以下 prompt，生成横版贴纸，再走渲染流程合成到外壳。

---

#### 输入

- 专辑封面图（任意尺寸正方形 PNG/JPEG）作为 image input 传入模型
- 从封面提取的主题色（dominant color）作为补充描述注入 prompt

---

#### Image Generation Prompt

```
A wide horizontal panoramic image with an aspect ratio of 12:5 (width to height).

This is a cinematic expansion of the provided album cover artwork. Do not simply
crop or tile the cover — instead, let the world of the cover breathe outward into
a wider landscape. The same mood, light, and emotional atmosphere should permeate
the entire canvas, but with more space, more air, more depth.

Style requirements:
- Seamlessly continue the color palette, texture, and visual language of the cover
- Painterly quality — evoke the feel of etching, layered watercolor, intaglio print,
  or aged fine art paper depending on the cover's aesthetic
- Atmospheric and immersive: open space, subtle gradients, organic texture
- The overall feeling should be expansive, intimate, and timeless

Composition rules — strictly follow these:
- If the cover has a portrait, figure, character, or any clear focal subject:
  place it on the RIGHT side of the canvas (right third), facing or opening toward
  the left. The left two-thirds should be open, atmospheric background
- If the cover has no clear subject (abstract, landscape, typographic):
  keep the visual weight evenly distributed but favor the right side for any
  concentrated detail
- The center horizontal band may be slightly softer/more diffuse, as it will
  be partially obscured by tape reel windows in the final composition
- If the original cover contains album title text or artist name typography,
  preserve or reinterpret it in the upper-right corner area of the panoramic
  image — keep it elegant, unobtrusive, and ensure it is not centered or low

Constraints:
- No new text or typography beyond what exists in the original cover
- No cassette tape mechanical parts (no reels, no spools, no tape window)
- No border, no frame, no vignette
- Fill the entire canvas edge to edge with no blank margins
- Pure artwork — not a product label or graphic design template
```

> 主色调不再注入 prompt，保留用于后续磁带外壳颜色着色功能。

---

#### 参考效果

`tape_core.png` 是 Nujabes《Modal Soul》的手工贴纸，体现了目标风格：
- 深红色为主色调铺满横版底色，是封面红色大地的延伸
- 深蓝色在顶部形成天空，封面宇宙感向上扩展
- 专辑中心意象以半透明草图形式隐约叠入，像记忆的残影
- 无任何文字，纯粹是封面世界的横向延展
- 整体有印刷做旧质感，像一张真实的老磁带标签

---

### 备用方案：纯算法生成（降级）

当绘图 API 不可用时使用，效果较弱：

| 方案 | 说明 |
|---|---|
| 封面居中 + 模糊延伸 | 封面居中清晰，两侧用高斯模糊放大版本填满 |
| 封面居中 + 主题色填两侧 | 从封面提取主色，纯色填充两侧留白区 |

---

## 外部工具依赖

| 工具 | 用途 | 获取方式 |
|---|---|---|
| `magick` (ImageMagick 7+) | PNG 合成 | `brew install imagemagick` |
| `etc1tool` | PNG → PKM | Android platform-tools |
| `pngquant` | ~~有损压缩~~（已弃用，影响画质）| — |

---

## 文件规格速查

| 项目 | 规格 |
|---|---|
| 磁带图尺寸 | 800×480 px |
| 标签贴纸区域 | 694×291 px，偏移 (54, 42) |
| 磁带图格式 | ETC1 PKM（推荐）或 JPEG（测试用）|
| 色深 | 8-bit sRGB |
| 外壳模板格式 | PNG + Alpha 通道 |
