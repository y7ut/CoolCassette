package preview

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// Info holds the metadata needed to render the preview HTML.
type Info struct {
	Artist     string
	Album      string
	TapeSlug   string
	ReelSlug   string
	TextColor  string  // "#FFFFFF" or "#000000"
	ArtistX    float64 // canvas coordinates (wampy config)
	ArtistY    float64
	TitleX     float64
	TitleY     float64
	AlbumX     float64 // negative = hidden
	AlbumY     float64
	TitleWidth float64 // max text width in px
}

// WriteHTML encodes tapePNGPath and reelPNGPath as base64 and writes a
// self-contained HTML preview to outPath.
func WriteHTML(tapePNGPath, reelPNGPath, outPath string, info Info) error {
	tapeData, err := os.ReadFile(tapePNGPath)
	if err != nil {
		return fmt.Errorf("read tape png: %w", err)
	}
	reelData, err := os.ReadFile(reelPNGPath)
	if err != nil {
		return fmt.Errorf("read reel png: %w", err)
	}

	tapeB64 := base64.StdEncoding.EncodeToString(tapeData)
	reelB64 := base64.StdEncoding.EncodeToString(reelData)

	html := buildHTML(tapeB64, reelB64, info)
	return os.WriteFile(outPath, []byte(html), 0644)
}

func buildHTML(tapeB64, reelB64 string, info Info) string {
	artist := info.Artist
	if artist == "" {
		artist = "Unknown Artist"
	}
	album := info.Album
	if album == "" {
		album = info.TapeSlug
	}

	// Text drawing config — passed into JS as literals
	textColor := info.TextColor
	if textColor == "" {
		textColor = "#000000"
	}
	artistX := info.ArtistX
	artistY := info.ArtistY
	titleX := info.TitleX
	titleY := info.TitleY
	albumX := info.AlbumX // negative = hidden
	albumY := info.AlbumY
	maxW := info.TitleWidth
	if maxW == 0 {
		maxW = 580
	}

	const fontSize = 31
	_ = strings.ToUpper

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s — %s</title>
<style>
@import url('https://fonts.googleapis.com/css2?family=VT323&family=Space+Mono&display=swap');
:root{--bg:#0a0a0a;--border:#2a2a2a;--accent:#c8f542;--dim:#444;--text:#e8e8e0;}
*{margin:0;padding:0;box-sizing:border-box;}
body{background:var(--bg);color:var(--text);font-family:'Space Mono',monospace;
  min-height:100vh;display:flex;flex-direction:column;align-items:center;
  justify-content:center;gap:32px;padding:40px 20px;}
body::before{content:'';position:fixed;inset:0;pointer-events:none;z-index:100;
  opacity:.5;background-image:url("data:image/svg+xml,%%3Csvg xmlns='http://www.w3.org/2000/svg' width='200' height='200'%%3E%%3Cfilter id='n'%%3E%%3CfeTurbulence type='fractalNoise' baseFrequency='.75' numOctaves='4' stitchTiles='stitch'/%%3E%%3C/filter%%3E%%3Crect width='200' height='200' filter='url(%%23n)' opacity='.04'/%%3E%%3C/svg%%3E");}
header{display:flex;align-items:baseline;gap:14px;}
.logo{font-family:'VT323',monospace;font-size:2.2rem;color:var(--accent);letter-spacing:2px;}
.tagline{font-size:.6rem;color:var(--dim);letter-spacing:.15em;text-transform:uppercase;}
.stage{position:relative;width:800px;height:480px;}
#tape-canvas{position:absolute;inset:0;width:800px;height:480px;display:block;}
#reel-canvas{position:absolute;left:180px;top:161px;width:440px;height:110px;pointer-events:none;}
.scanlines{position:absolute;inset:0;pointer-events:none;
  background:repeating-linear-gradient(0deg,transparent,transparent 2px,rgba(0,0,0,.04) 2px,rgba(0,0,0,.04) 4px);}
.controls{display:flex;flex-direction:column;align-items:center;gap:18px;width:800px;}
.track-info{display:flex;justify-content:space-between;width:100%%;}
.track-name{font-size:.65rem;letter-spacing:.1em;text-transform:uppercase;
  overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:680px;}
.btn-row{display:flex;gap:10px;align-items:center;}
button{background:none;border:1px solid var(--border);color:var(--dim);
  font-family:'Space Mono',monospace;font-size:.58rem;letter-spacing:.15em;
  text-transform:uppercase;padding:7px 18px;cursor:pointer;transition:all .12s;}
button:hover{border-color:var(--accent);color:var(--accent);box-shadow:0 0 10px rgba(200,245,66,.12);}
#btn-play{font-size:1.2rem;padding:9px 24px;border-color:var(--accent);color:var(--accent);letter-spacing:0;}
#btn-play:hover{background:rgba(200,245,66,.07);}
.sep{width:1px;height:26px;background:var(--border);}
.speed-label{font-size:.52rem;color:var(--dim);letter-spacing:.1em;}
.status{font-size:.56rem;color:var(--dim);letter-spacing:.12em;text-transform:uppercase;display:flex;gap:24px;}
.status span{color:var(--text);}
</style>
</head>
<body>
<header>
  <div class="logo">COOLCASSETTE</div>
  <div class="tagline">tape preview</div>
</header>
<div class="stage">
  <!-- tape-canvas renders tape image + text labels -->
  <canvas id="tape-canvas" width="800" height="480"></canvas>
  <canvas id="reel-canvas" width="440" height="110"></canvas>
  <div class="scanlines"></div>
</div>
<div class="controls">
  <div class="track-info">
    <div class="track-name">%s</div>
  </div>
  <div class="btn-row">
    <button id="btn-play">▶</button>
    <div class="sep"></div>
    <span class="speed-label">SPEED</span>
    <button id="btn-slow">× 0.5</button>
    <button id="btn-norm">× 1</button>
    <button id="btn-fast">× 2</button>
    <div class="sep"></div>
    <span class="speed-label">LABELS</span>
    <button id="btn-labels">Labels</button>
  </div>
</div>
<div class="status">
  <div>FRAME <span id="stat-frame">00</span> / 40</div>
  <div>DELAY <span id="stat-delay">55</span> ms</div>
  <div>STATUS <span id="stat-status">STOPPED</span></div>
</div>
<script>
// ── reel atlas config ─────────────────────────────────────────────────────
const ATLAS_COLS=4,FRAME_W=440,FRAME_H=110,FRAME_COUNT=40,BASE_DELAY=55;
const FRAMES=Array.from({length:FRAME_COUNT},(_,i)=>({
  x:(i%%ATLAS_COLS)*FRAME_W, y:Math.floor(i/ATLAS_COLS)*FRAME_H
}));
const CIRCLES=[{cx:57,cy:56,r:42},{cx:383,cy:56,r:42}];

// ── text label config (from wampy config.txt) ─────────────────────────────
const ARTIST      = %q;
const ALBUM_NAME  = %q;
const TEXT_COLOR  = %q;
const ARTIST_X    = %.1f, ARTIST_Y = %.1f;
const TITLE_X     = %.1f, TITLE_Y  = %.1f;
const ALBUM_X     = %.1f, ALBUM_Y  = %.1f;  // negative = hidden
const MAX_W       = %.1f;
const FONT_SIZE   = %d;

// ── state ─────────────────────────────────────────────────────────────────
let atlasImg=null, tapeImg=null;
let frameIdx=0, playing=false, lastTick=0, delay=BASE_DELAY;
let showLabels=true;

const tapeCanvas = document.getElementById('tape-canvas');
const tapeCtx    = tapeCanvas.getContext('2d');
const reelCanvas = document.getElementById('reel-canvas');
const reelCtx    = reelCanvas.getContext('2d');
const btnPlay    = document.getElementById('btn-play');
const statFr     = document.getElementById('stat-frame');
const statDel    = document.getElementById('stat-delay');
const statSt     = document.getElementById('stat-status');

// ── text layout constants ─────────────────────────────────────────────────

function truncateText(ctx, text, maxW){
  if(ctx.measureText(text).width <= maxW) return text;
  let lo=0, hi=text.length;
  while(lo < hi-1){
    const mid = (lo+hi)>>1;
    if(ctx.measureText(text.slice(0,mid)+'…').width <= maxW) lo=mid; else hi=mid;
  }
  return text.slice(0,lo)+'…';
}

// ── draw tape + text labels ───────────────────────────────────────────────
function drawTape(){
  if(!tapeImg) return;
  tapeCtx.clearRect(0,0,800,480);
  tapeCtx.drawImage(tapeImg,0,0,800,480);

  if(!showLabels) return;

  tapeCtx.font = FONT_SIZE+'px sans-serif';
  tapeCtx.fillStyle = TEXT_COLOR;
  tapeCtx.textBaseline = 'alphabetic';
  // Place text so the first line starts 80px from the top of the tape (800×480).
  // config.txt Y coords are approximate; we override with fixed offset from top.
  const ARTIST_TOP = 100;                      // artist baseline from tape top
  const LINE_GAP   = FONT_SIZE + 14;           // gap between artist and title lines

  // artist
  if(ARTIST_X >= 0){
    const t = truncateText(tapeCtx, ARTIST, MAX_W);
    tapeCtx.fillText(t, ARTIST_X, ARTIST_TOP);
  }
  // title (album name used as title for preview)
  if(TITLE_X >= 0){
    const t = truncateText(tapeCtx, ALBUM_NAME, MAX_W);
    tapeCtx.fillText(t, TITLE_X, ARTIST_TOP + LINE_GAP);
  }
  // album (usually hidden, albumx < 0)
  if(ALBUM_X >= 0){
    const t = truncateText(tapeCtx, ALBUM_NAME, MAX_W);
    tapeCtx.fillText(t, ALBUM_X, ARTIST_TOP + LINE_GAP * 2);
  }
}

function truncateText(ctx, text, maxW){
  if(ctx.measureText(text).width <= maxW) return text;
  let lo=0, hi=text.length;
  while(lo < hi-1){
    const mid = (lo+hi)>>1;
    if(ctx.measureText(text.slice(0,mid)+'…').width <= maxW) lo=mid; else hi=mid;
  }
  return text.slice(0,lo)+'…';
}

// ── draw reel frame ───────────────────────────────────────────────────────
function drawReel(idx){
  if(!atlasImg) return;
  const f=FRAMES[idx];
  reelCtx.clearRect(0,0,FRAME_W,FRAME_H);
  reelCtx.save();
  reelCtx.beginPath();
  for(const c of CIRCLES){reelCtx.moveTo(c.cx+c.r,c.cy);reelCtx.arc(c.cx,c.cy,c.r,0,Math.PI*2);}
  reelCtx.clip();
  reelCtx.drawImage(atlasImg,f.x,f.y,FRAME_W,FRAME_H,0,0,FRAME_W,FRAME_H);
  reelCtx.restore();
}

// ── animation loop ────────────────────────────────────────────────────────
function tick(ts){
  if(!playing) return;
  if(ts-lastTick >= delay){
    frameIdx=(frameIdx+1)%%FRAME_COUNT;
    drawReel(frameIdx);
    lastTick=ts;
  }
  requestAnimationFrame(tick);
}

function updateUI(){
  statFr.textContent=String(frameIdx).padStart(2,'0');
  statDel.textContent=delay;
  statSt.textContent=playing?'PLAYING':'STOPPED';
  btnPlay.textContent=playing?'⏸':'▶';
}
function toggleLabels(){
  showLabels=!showLabels;
  drawTape();
  const btnLabels=document.getElementById('btn-labels');
  if(showLabels){
    btnLabels.style.borderColor='var(--border)';
    btnLabels.style.color='var(--dim)';
    btnLabels.style.boxShadow='none';
    btnLabels.textContent='Hide';
  }else{
    btnLabels.style.borderColor='var(--accent)';
    btnLabels.style.color='var(--accent)';
    btnLabels.style.boxShadow='0 0 10px rgba(200,245,66,.12)';
    btnLabels.textContent='Show';
  }
}
function startPlay(){if(playing)return;playing=true;lastTick=performance.now();updateUI();requestAnimationFrame(tick);}
function stopPlay(){playing=false;updateUI();}
function togglePlay(){playing?stopPlay():startPlay();}

btnPlay.addEventListener('click',togglePlay);
document.getElementById('btn-labels').addEventListener('click',toggleLabels);
document.getElementById('btn-slow').addEventListener('click',()=>{delay=BASE_DELAY*2;updateUI();});
document.getElementById('btn-norm').addEventListener('click',()=>{delay=BASE_DELAY;updateUI();});
document.getElementById('btn-fast').addEventListener('click',()=>{delay=Math.round(BASE_DELAY/2);updateUI();});
document.addEventListener('keydown',e=>{if(e.code==='Space'){e.preventDefault();togglePlay();}});

// ── load images ───────────────────────────────────────────────────────────
function loadImg(src){ return new Promise((res,rej)=>{ const i=new Image(); i.onload=()=>res(i); i.onerror=rej; i.src=src; }); }

(async()=>{
  [tapeImg, atlasImg] = await Promise.all([
    loadImg('data:image/png;base64,%s'),
    loadImg('data:image/png;base64,%s'),
  ]);
  drawTape();
  drawReel(0);
  updateUI();
})();
</script>
</body>
</html>`,
		artist, album,
		// bottom track-name div
		strings.ToUpper(album+" — "+artist),
		// text label JS literals
		artist, album, textColor,
		artistX, artistY,
		titleX, titleY,
		albumX, albumY,
		maxW, fontSize,
		// base64 images
		tapeB64, reelB64,
	)
}
