package reel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Params defines the geometry for extracting and rotating reel circles from a tape PNG.
//
// The tape image is 800×480. The reel template region is a 440×110 crop starting at
// (180, 161). Within that template:
//   - Left reel center:  (57, 56), radius 42  → crop origin (15, 14)
//   - Right reel center: (383, 56), radius 42 → crop origin (341, 14)
//
// Each reel circle is 84×84 px. 40 frames at 9° per frame = full 360° rotation.
// Final atlas: 4 columns × 10 rows = 1760×1100 px.
type Params struct {
	// Crop of the reel band from the full tape image
	TemplateX, TemplateY int // top-left of reel template region in tape image
	TemplateW, TemplateH int // size of reel template

	// Left reel circle within the template
	LeftCX, LeftCY int // center
	// Right reel circle within the template
	RightCX, RightCY int // center

	CircleRadius int // radius of each reel circle (diameter = 2*r)

	FrameCount  int // total animation frames
	DegreesStep int // rotation degrees per frame
	AtlasCols   int // columns in the output atlas
}

// DefaultParams returns the calibrated geometry for CoolCassette tape skins.
func DefaultParams() Params {
	return Params{
		TemplateX: 180, TemplateY: 161,
		TemplateW: 440, TemplateH: 110,
		LeftCX: 57, LeftCY: 56,
		RightCX: 383, RightCY: 56,
		CircleRadius: 42,
		FrameCount:   40,
		DegreesStep:  9,
		AtlasCols:    4,
	}
}

// Generate creates reel.png from tapePNGPath and writes it to outPath.
// If outPath already exists it is returned immediately (cached).
func Generate(tapePNGPath, outPath string) error {
	return GenerateWithParams(tapePNGPath, outPath, DefaultParams())
}

// BuildAtlas compresses reel.png into atlas.pkm (ETC1) and writes atlas.txt + config.txt
// into outDir, ready for deployment to wampy.
// animDelayMS is the per-frame delay in milliseconds (wampy default is 55ms).
func BuildAtlas(reelPNGPath, outDir, etc1toolPath string, p Params, animDelayMS int) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("reel atlas: create dir: %w", err)
	}

	// 1. Compress reel.png → atlas.pkm
	atlasPKM := filepath.Join(outDir, "atlas.pkm")
	cmd := exec.Command(etc1toolPath, reelPNGPath, "-o", atlasPKM)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("reel atlas: etc1tool: %w\n%s", err, string(out))
	}

	// 2. Write atlas.txt — one line per frame: <x> <y> <width> <height>
	//    Frames are laid out left-to-right, top-to-bottom in AtlasCols columns.
	var sb strings.Builder
	for i := 0; i < p.FrameCount; i++ {
		col := i % p.AtlasCols
		row := i / p.AtlasCols
		x := col * p.TemplateW
		y := row * p.TemplateH
		sb.WriteString(fmt.Sprintf("%d %d %d %d\n", x, y, p.TemplateW, p.TemplateH))
	}
	if err := os.WriteFile(filepath.Join(outDir, "atlas.txt"), []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("reel atlas: write atlas.txt: %w", err)
	}

	// 3. Write config.txt — animation delay
	cfgContent := fmt.Sprintf("delayMS: %d\n", animDelayMS)
	if err := os.WriteFile(filepath.Join(outDir, "config.txt"), []byte(cfgContent), 0644); err != nil {
		return fmt.Errorf("reel atlas: write config.txt: %w", err)
	}

	return nil
}

// GenerateWithParams is the same as Generate but accepts custom geometry.
func GenerateWithParams(tapePNGPath, outPath string, p Params) error {
	// Cache check
	if _, err := os.Stat(outPath); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("reel: create output dir: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "coolcassette-reel-*")
	if err != nil {
		return fmt.Errorf("reel: create tmp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	diameter := p.CircleRadius * 2

	// Step 1: crop the reel template strip from the tape image
	templatePath := filepath.Join(tmpDir, "template.png")
	if err := magickRun(
		tapePNGPath,
		"-crop", fmt.Sprintf("%dx%d+%d+%d", p.TemplateW, p.TemplateH, p.TemplateX, p.TemplateY),
		"+repage",
		templatePath,
	); err != nil {
		return fmt.Errorf("reel: crop template: %w", err)
	}

	// Step 2: build a circular mask (white circle on transparent background)
	maskPath := filepath.Join(tmpDir, "mask.png")
	if err := magickRun(
		"-size", fmt.Sprintf("%dx%d", diameter, diameter),
		"xc:none",
		"-fill", "white",
		"-draw", fmt.Sprintf("circle %d,%d %d,0", p.CircleRadius, p.CircleRadius, p.CircleRadius),
		maskPath,
	); err != nil {
		return fmt.Errorf("reel: create mask: %w", err)
	}

	// Left circle crop origin inside template
	leftX := p.LeftCX - p.CircleRadius
	leftY := p.LeftCY - p.CircleRadius
	// Right circle crop origin inside template
	rightX := p.RightCX - p.CircleRadius
	rightY := p.RightCY - p.CircleRadius

	// Step 3: extract each circle with circular mask applied
	leftCirclePath := filepath.Join(tmpDir, "left_circle.png")
	rightCirclePath := filepath.Join(tmpDir, "right_circle.png")

	if err := applyCircleMask(templatePath, maskPath, leftX, leftY, diameter, leftCirclePath); err != nil {
		return fmt.Errorf("reel: left circle: %w", err)
	}
	if err := applyCircleMask(templatePath, maskPath, rightX, rightY, diameter, rightCirclePath); err != nil {
		return fmt.Errorf("reel: right circle: %w", err)
	}

	// Step 4: generate rotated frames
	framePaths := make([]string, p.FrameCount)
	for i := 0; i < p.FrameCount; i++ {
		angle := i * p.DegreesStep
		leftRot := filepath.Join(tmpDir, fmt.Sprintf("left_%02d.png", i))
		rightRot := filepath.Join(tmpDir, fmt.Sprintf("right_%02d.png", i))

		if err := rotateCircle(leftCirclePath, angle, diameter, leftRot); err != nil {
			return fmt.Errorf("reel: rotate left frame %d: %w", i, err)
		}
		if err := rotateCircle(rightCirclePath, angle, diameter, rightRot); err != nil {
			return fmt.Errorf("reel: rotate right frame %d: %w", i, err)
		}

		framePath := filepath.Join(tmpDir, fmt.Sprintf("frame_%02d.png", i))
		if err := compositeFrame(templatePath, leftRot, rightRot, leftX, leftY, rightX, rightY, framePath); err != nil {
			return fmt.Errorf("reel: composite frame %d: %w", i, err)
		}
		framePaths[i] = framePath
	}

	// Step 5: stitch frames into atlas (AtlasCols columns)
	rows := p.FrameCount / p.AtlasCols
	rowPaths := make([]string, rows)
	for r := 0; r < rows; r++ {
		rowArgs := []string{}
		for c := 0; c < p.AtlasCols; c++ {
			rowArgs = append(rowArgs, framePaths[r*p.AtlasCols+c])
		}
		rowPath := filepath.Join(tmpDir, fmt.Sprintf("row_%02d.png", r))
		rowArgs = append(rowArgs, "+append", rowPath)
		if err := magickRun(rowArgs...); err != nil {
			return fmt.Errorf("reel: append row %d: %w", r, err)
		}
		rowPaths[r] = rowPath
	}

	// Append all rows vertically → final atlas
	finalArgs := append(rowPaths, "-append", outPath)
	if err := magickRun(finalArgs...); err != nil {
		return fmt.Errorf("reel: assemble atlas: %w", err)
	}

	return nil
}

// applyCircleMask crops a circle from src at (ox,oy) with given diameter and applies mask.
func applyCircleMask(templatePath, maskPath string, ox, oy, diameter int, outPath string) error {
	return magickRun(
		templatePath,
		"-crop", fmt.Sprintf("%dx%d+%d+%d", diameter, diameter, ox, oy),
		"+repage",
		maskPath, "-alpha", "on", "-compose", "CopyOpacity", "-composite",
		outPath,
	)
}

// rotateCircle rotates a circle image by angle degrees, keeping extent fixed.
func rotateCircle(circlePath string, angle, diameter int, outPath string) error {
	return magickRun(
		circlePath,
		"-background", "none",
		"-rotate", fmt.Sprintf("%d", angle),
		"-gravity", "center",
		"-extent", fmt.Sprintf("%dx%d", diameter, diameter),
		outPath,
	)
}

// compositeFrame composites two rotated circles onto the template for one frame.
func compositeFrame(templatePath, leftRot, rightRot string, lx, ly, rx, ry int, outPath string) error {
	return magickRun(
		templatePath,
		leftRot, "-geometry", fmt.Sprintf("+%d+%d", lx, ly), "-composite",
		rightRot, "-geometry", fmt.Sprintf("+%d+%d", rx, ry), "-composite",
		outPath,
	)
}

var magickBin = "magick"

func SetMagickPath(p string) { magickBin = p }

// magickRun executes ImageMagick with the given arguments.
func magickRun(args ...string) error {
	cmd := exec.Command(magickBin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("magick %v: %w\n%s", args[0], err, string(out))
	}
	return nil
}
