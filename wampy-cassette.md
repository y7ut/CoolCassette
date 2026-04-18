
### Custom skins

Device directory tree:

```shell
LEARNING/
MUSIC/
wampy/
├── config.ini
└── skins
    ├── cassette
    │   ├── reel
    │   │   └── test_reel
    │   │       ├── ic_audio_play_tape_reel_other_32.jpg
    │   │       └── ic_audio_play_tape_reel_other_33.jpg
    │   └── tape
    │       ├── ccc
    │       │   └── c.jpg
    │       └── test_tape
    │           ├── cas.jpg
    │           └── config.txt
    └── winamp
        ├── Winamp3_Classified_v5.5.wsz
        ├── Winamp5_Classified_v5.5.wsz
        └── windows98.wsz
```

Winamp has 3 custom skins, cassette has one custom reel with 2 frames and two tapes. Tape `ccc` uses default config.

`wampy` directory is automatically created on internal storage (*not* SD card).

#### Cassette skins

Tape format: JPEG, 800x480 to cover whole screen, any name, `.jpg` extension. Only first found file in directory is
used. You can get some nice tapes from http://tapedeck.org/, non-transparent ones work well with `other` reel.

Reel format: JPEG, any size, any name. All found files in directory are used. Position is defined by tape
in `config.txt`.

Reel sprite changes every 55 ms. Default reels have 57 images each.

`config.txt` contents:

```yaml
# default reel
reel: other
# track artist coordinates
artistx: 83.0
artisty: 82.0
# artist format string
artistformat: $ARTIST
# track title coordinates
titlex: 83.0
titley: 117.0
# track format string
titleformat: $TITLE
# album coordinates, hidden
albumx: -1.0
albumy: -1.0
# album format string
albumformat: $ALBUM
# reel upper left coordinate
reelx: 134.0
reely: 160.0
# max line width in pixels, title, album and artist will be cut after that value
titlewidth: 600.0
# zero-padded minutes and zero-padded seconds separated by colon
durationformat: %1$02d:%2$02d
# text color, RGB
textcolor: #000000
```

Remember, `(0,0)` is top left corner.

Set `artistx`/`titlex`/`albumx` to negative value to hide artist/title/album labels.

Format variables:

- `$title` / `$TITLE` (as is / uppercase)
- `$artist` / `$ARTIST`
- `$album` / `$ALBUM`
- `$year`
- `$track`
- `$duration`

Format options may be omitted; default values will take their place instead.

Duration format
follows [printf syntax](https://man7.org/linux/man-pages/man3/fprintf.3.html), `Format of the format string` section.
Default value `%1$02d:%2$02d` prints duration in `mm:ss` format. Arguments: minute, second, hour.
Example: `%3$01d:%1$02d:%2$02d` will print `2:06:37` for track 2 hours, 06 minutes, 37 seconds long.

Config file is not required; default one (with values above) will be used instead.

Example [tape](./images/ic_audio_play_cassette_ahf_picture.jpg), [reel](./images/ic_audio_play_tape_reel_other_00.jpg).

⚠️**WARNING**⚠️

GPU memory is shared with main memory and usually there is not much left. Using huge images as tapes/reels
is a **bad** idea. Too many reel textures is bad too.
Consult [What to do if device crashes / Wampy doesn't start?](#what-to-do-if-device-crashes--Wampy-doesnt-start)
section.

Alternatively, see advanced section below.

#### Cassette skins, advanced

JPEG-based skins are very slow to load and take a lot of memory. You should use compressed textures (ETC1, `.pkm`
extension) and atlases.

File naming:

```text
wampy/
├── config.ini
└── skins
    ├── reel
    │   └── awesomeReel
    │       ├── atlas.pkm
    │       ├── atlas.txt
    │       └── config.txt
    └── tape
        └── myCoolTape
            ├── config.txt
            └── tape.pkm
```

ETC1 textures are produced from PNG by etc1tool - [Windows][1], [Mac][2], [Linux][3]

[1]: https://dl.google.com/android/repository/platform-tools-latest-windows.zip

[2]: https://dl.google.com/android/repository/platform-tools-latest-darwin.zip

[3]: https://dl.google.com/android/repository/platform-tools-latest-linux.zip

Atlas contains all the reel images. Maximum resolution: 4096x4096.

Creating atlas, Linux, ImageMagick installed:

```shell
# 4-column atlas with all reel images
montage -mode concatenate -tile 4x reel_source_dir/*.jpg reel_source_dir/atlas.png
# create reel_source_dir/atlas.pkm
etc1tool reel_source_dir/atlas.png
```

After creating pkm file you need to produce `atlas.txt`. This file contains coordinates for your reel tiles.

Format:

```text
<x> <y> <width> <height>
```

Example:

[Atlas image](./images/example-atlas.jpg)

`atlas.txt`:

```text
0 0 528 116
528 0 528 116
1056 0 528 116
1584 0 528 116
0 116 528 116
```

Compressed atlases support configurable animation delay. Put `config.txt` along with atlas files.

Contents:

```text
delayMS: 100
```

#### Cassette skins per directory

You can force a tape/reel combination for a specific directory by placing `cassette.txt` in that directory with
contents:

```text
tape: <tape name>
reel: <reel name>
```

For example, if you want to display tape `chf` with reel `other` when a track from directory `2000 - Into the Abyss`
plays, you put `cassette.txt` into that directory. Contents:

```text
tape: chf
reel: other
```
