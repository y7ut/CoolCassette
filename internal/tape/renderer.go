package tape

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coolcassette/coolcassette/internal/theme"
	"google.golang.org/genai"
)

var magickBin = "magick"

func SetMagickPath(p string) { magickBin = p }

var customHTTPClient = &http.Client{
	Timeout: 300 * time.Second,
}

const (
	LabelX      = 54
	LabelY      = 42
	LabelWidth  = 694
	LabelHeight = 291

	CanvasWidth  = 800
	CanvasHeight = 480
)

// Provider selects the image generation backend.
type Provider string

const (
	ProviderCustom Provider = "custom"
	ProviderGoogle Provider = "google"
)

// Options controls tape rendering behavior.
type Options struct {
	Shell    string // "chf" or "bhf"
	APIKey   string
	Provider Provider // "custom" or "google"
	BaseURL  string   // custom OpenAI-compatible base URL
	Model    string   // model name for custom provider
	Verbose  bool
}

// RenderResult holds the output paths after rendering.
type RenderResult struct {
	PNGPath string
	PKMPath string
}

// RenderShellGuided generates a tape PNG using the "shell-guided" method:
// both the album cover and the cassette shell template are passed to the AI,
// which outputs a ready-to-use 800×480 tape image directly.
// The AI can color the shell body based on the cover's palette and place the
// artwork precisely in the label window without post-composite cropping issues.
//
// If cachedPNGPath exists, the API call is skipped (same cache logic as Render).
func RenderShellGuided(
	ctx context.Context,
	coverData []byte,
	colors []theme.Color,
	outDir string,
	shellsDir string,
	etc1toolPath string,
	cachedPNGPath string,
	opts Options,
) (*RenderResult, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	pngPath := filepath.Join(outDir, "tape.png")

	// Check if a pre-generated PNG is available
	if cachedPNGPath != "" {
		if _, err := os.Stat(cachedPNGPath); err == nil {
			if opts.Verbose {
				fmt.Printf("  [cache] reusing existing tape PNG: %s\n", cachedPNGPath)
			}
			if cachedPNGPath != pngPath {
				if err := copyFile(cachedPNGPath, pngPath); err != nil {
					return nil, fmt.Errorf("copy cached png: %w", err)
				}
			}
			goto encodeShellPKM
		}
	}

	// Generate full tape image with shell context
	{
		shellPath := filepath.Join(shellsDir, fmt.Sprintf("shell_%s.png", opts.Shell))
		shellData, err := os.ReadFile(shellPath)
		if err != nil {
			return nil, fmt.Errorf("read shell template: %w", err)
		}

		prompt := buildShellGuidedPrompt(colors)
		if opts.Verbose {
			fmt.Printf("[tape/shell-guided] provider: %s\n", opts.Provider)
			fmt.Printf("[tape/shell-guided] prompt: %s\n", prompt)
		}

		var imgBytes []byte
		switch opts.Provider {
		case ProviderGoogle:
			imgBytes, err = generateViaGoogleGenAI(ctx, coverData, shellData, prompt, opts.APIKey)
		default:
			imgBytes, err = generateViaCustomOpenAI(ctx, coverData, shellData, prompt, opts)
		}
		if err != nil {
			return nil, fmt.Errorf("generate shell-guided image: %w", err)
		}

		// Write and resize to exact 800×480
		tmpRaw := pngPath + ".raw.png"
		if err := os.WriteFile(tmpRaw, imgBytes, 0644); err != nil {
			return nil, err
		}
		defer os.Remove(tmpRaw)

		cmd := exec.CommandContext(ctx, magickBin, tmpRaw,
			"-resize", fmt.Sprintf("%dx%d!", CanvasWidth, CanvasHeight),
			"-depth", "8",
			pngPath,
		)
		hideWindow(cmd)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("resize tape image: %w\n%s", err, string(out))
		}
	}

encodeShellPKM:
	pkmPath := filepath.Join(outDir, "tape.pkm")
	if err := encodePKM(etc1toolPath, pngPath, pkmPath); err != nil {
		return nil, fmt.Errorf("encode pkm: %w", err)
	}

	// NOTE: tape.png is intentionally kept so the caller can use it (e.g. reel generation).
	// The caller is responsible for removing it when no longer needed.
	return &RenderResult{
		PNGPath: pngPath,
		PKMPath: pkmPath,
	}, nil
}

// RenderPreviewShellGuided generates only a tape.png using the shell-guided method.
func RenderPreviewShellGuided(
	ctx context.Context,
	coverData []byte,
	colors []theme.Color,
	outPath string,
	shellsDir string,
	opts Options,
) error {
	shellPath := filepath.Join(shellsDir, fmt.Sprintf("shell_%s.png", opts.Shell))
	shellData, err := os.ReadFile(shellPath)
	if err != nil {
		return fmt.Errorf("read shell template: %w", err)
	}

	prompt := buildShellGuidedPrompt(colors)
	if opts.Verbose {
		fmt.Printf("[tape/shell-guided] prompt: %s\n", prompt)
	}

	var imgBytes []byte
	switch opts.Provider {
	case ProviderGoogle:
		imgBytes, err = generateViaGoogleGenAI(ctx, coverData, shellData, prompt, opts.APIKey)
	default:
		imgBytes, err = generateViaCustomOpenAI(ctx, coverData, shellData, prompt, opts)
	}
	if err != nil {
		return fmt.Errorf("generate shell-guided image: %w", err)
	}

	tmpRaw := outPath + ".raw.png"
	if err := os.WriteFile(tmpRaw, imgBytes, 0644); err != nil {
		return err
	}
	defer os.Remove(tmpRaw)

	cmd := exec.CommandContext(ctx, magickBin, tmpRaw,
		"-resize", fmt.Sprintf("%dx%d!", CanvasWidth, CanvasHeight),
		"-depth", "8",
		outPath,
	)
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("resize tape image: %w\n%s", err, string(out))
	}
	return nil
}

// FindCoreTape looks for a manually provided sticker image in dir.
// Accepted filenames: core_tape.png, core_tape.jpg, core_tape.jpeg (case-insensitive).
// Returns the full path if found, or "" if not present.
func FindCoreTape(dir string) string {
	candidates := []string{
		"core_tape.png",
		"core_tape.jpg",
		"core_tape.jpeg",
		"core_tape.PNG",
		"core_tape.JPG",
		"core_tape.JPEG",
	}
	for _, name := range candidates {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			fmt.Printf("  [core tape] found: %s\n", p)
			return p
		}
	}
	return ""
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// encodePKM compresses a PNG to ETC1 PKM format using etc1tool.
func encodePKM(etc1toolPath, pngPath, pkmPath string) error {
	cmd := exec.Command(etc1toolPath, pngPath, "-o", pkmPath)
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("etc1tool: %w\n%s", err, string(out))
	}
	return nil
}

// decodeDataURL extracts raw bytes from a base64 data URL.
// Handles both "data:image/png;base64,<data>" and plain base64.
func decodeDataURL(dataURL string) ([]byte, error) {
	if idx := strings.Index(dataURL, ","); idx >= 0 {
		return base64.StdEncoding.DecodeString(dataURL[idx+1:])
	}
	return base64.StdEncoding.DecodeString(dataURL)
}

func extractImageFromContent(raw json.RawMessage) []byte {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if imgData, err := extractBase64ImageFromMarkdown(s); err == nil {
			return imgData
		}
	}
	var parts []struct {
		Type     string `json:"type"`
		ImageURL *struct {
			URL string `json:"url"`
		} `json:"image_url"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		for _, p := range parts {
			if p.Type == "image_url" && p.ImageURL != nil && p.ImageURL.URL != "" {
				if imgData, err := decodeDataURL(p.ImageURL.URL); err == nil {
					return imgData
				}
			}
		}
	}
	return nil
}

func extractBase64ImageFromMarkdown(content string) ([]byte, error) {
	start := strings.Index(content, "![image](")
	if start < 0 {
		return nil, fmt.Errorf("no markdown image found")
	}
	urlStart := start + len("![image](")
	end := strings.IndexByte(content[urlStart:], ')')
	if end < 0 {
		return nil, fmt.Errorf("malformed markdown image")
	}
	dataURL := content[urlStart : urlStart+end]
	return decodeDataURL(dataURL)
}

func downloadURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// buildShellGuidedPrompt constructs the prompt for the shell-guided method.
// The AI receives both the album cover AND the cassette shell template,
// and is asked to paint the complete 800×480 tape image.
func buildShellGuidedPrompt(_ []theme.Color) string {
	return strings.TrimSpace(`
You are given two images:
  1. An album cover (square artwork)
  2. A cassette tape shell template (800×480 px) — a plastic body with a
     rectangular transparent window in the upper area where the paper label sits,
     and two circular reel windows in the lower-center area.

Your task: produce a single finished 800×480 cassette tape image.

=== LABEL AREA (the transparent rectangle in the template) ===
Fill this area with a cinematic panoramic expansion of the album cover:
- Extend the cover's mood, color palette, light, and texture outward into a
  wide horizontal landscape — do not simply crop or repeat the cover
- Painterly quality that matches the cover's aesthetic (etching, watercolor,
  photography, illustration — match the source)
- If the cover has a portrait, figure, or clear focal subject:
  place it on the RIGHT third of the label, facing/opening toward the left.
  The left two-thirds should be open, atmospheric background
- The center horizontal band of the label may be slightly softer/more diffuse
  (the reel windows partially overlap this zone in the final display)
- If the original cover has album title or artist text, reinterpret it subtly
  in the upper-right of the label — elegant, small, not centered
- No new text beyond what exists in the cover
- No cassette mechanical parts drawn inside the label area

=== REEL WINDOWS (the two circular openings in the shell) ===
The reel circles will be cut out and spun as a looping animation, so they must be
completely shadow-free and evenly lit — no drop shadows, no cast shadows, no
directional lighting, no dark edges or gradients of any kind inside the circles.

=== SHELL BODY (everything outside the label window) ===
- Recolor the cassette shell body to harmonize with the album cover's dominant
  palette — choose a single bold, saturated, or deeply tonal color that feels
  like it belongs to the same artistic universe as the cover
- Keep all physical details of the shell (screws, ridges, reel windows, brand
  indentations) visible — just recolor the plastic surface
- The two reel windows should remain as dark/transparent circles — do not
  fill them with artwork
- The overall shell color should contrast enough with the label art so the
  cassette reads as a physical object

=== FINAL CONSTRAINTS ===
- Output exactly 800×480 pixels, filled edge to edge
- No border, no vignette, no drop shadow, no white margins
- Text from the cassette tape shell template  should not appear in the final
- The result should look like a real, physical cassette tape you could hold`)
}

// generateViaGoogleGenAI uses the official Google GenAI SDK to generate a tape image.
func generateViaGoogleGenAI(ctx context.Context, coverData, shellData []byte, prompt, apiKey string) ([]byte, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Google API key not set (use --api-key or GOOGLE_API_KEY env)")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("genai client: %w", err)
	}

	parts := []*genai.Part{
		genai.NewPartFromText(prompt),
		{InlineData: &genai.Blob{MIMEType: "image/jpeg", Data: coverData}},
		{InlineData: &genai.Blob{MIMEType: "image/png", Data: shellData}},
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	result, err := client.Models.GenerateContent(ctx, "gemini-3.1-flash-image-preview", contents, nil)
	if err != nil {
		return nil, fmt.Errorf("genai generate: %w", err)
	}

	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in Google GenAI response")
	}

	for _, part := range result.Candidates[0].Content.Parts {
		if part.InlineData != nil && len(part.InlineData.Data) > 0 {
			return part.InlineData.Data, nil
		}
	}

	return nil, fmt.Errorf("no image in Google GenAI response")
}

func generateViaCustomOpenAI(ctx context.Context, coverData, shellData []byte, prompt string, opts Options) ([]byte, error) {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("CUSTOM_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key not set (use --api-key or CUSTOM_API_KEY env)")
	}

	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("CUSTOM_BASE_URL")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("base URL not set (use --base-url or CUSTOM_BASE_URL env)")
	}

	model := opts.Model
	if model == "" {
		model = os.Getenv("CUSTOM_MODEL")
	}
	if model == "" {
		return nil, fmt.Errorf("model not set (use --model or CUSTOM_MODEL env)")
	}

	coverB64 := base64.StdEncoding.EncodeToString(coverData)
	shellB64 := base64.StdEncoding.EncodeToString(shellData)

	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "text",
						"text": "Image 1 — Album cover:",
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": "data:image/jpeg;base64," + coverB64,
						},
					},
					{
						"type": "text",
						"text": "Image 2 — Cassette shell template (800×480, label area is transparent/cut out):",
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": "data:image/png;base64," + shellB64,
						},
					},
					{
						"type": "text",
						"text": prompt,
					},
				},
			},
		},
		"modalities": []string{"image", "text"},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"
	apiCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 300*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(apiCtx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := customHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, string(respData))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
				Images  []struct {
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"images"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w\n%s", err, string(respData))
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no image in response: %s", string(respData))
	}

	msg := result.Choices[0].Message
	if len(msg.Images) > 0 && msg.Images[0].ImageURL.URL != "" {
		return decodeDataURL(msg.Images[0].ImageURL.URL)
	}
	if len(msg.Content) > 0 {
		if imgData := extractImageFromContent(msg.Content); imgData != nil {
			return imgData, nil
		}
	}
	return nil, fmt.Errorf("no image in response: %s", string(respData))
}

// CompositeTapePublic composites a sticker/core_tape image onto the 800×480 shell template.
// Used by the core_tape manual override path.
func CompositeTapePublic(stickerPath, shellPath, outPath string) error {
	cmd := exec.Command(magickBin,
		"-size", fmt.Sprintf("%dx%d", CanvasWidth, CanvasHeight),
		"xc:#000000",
		"(", stickerPath, "-resize", fmt.Sprintf("%dx%d!", LabelWidth, LabelHeight), ")",
		"-geometry", fmt.Sprintf("+%d+%d", LabelX, LabelY),
		"-composite",
		shellPath,
		"-composite",
		"-depth", "8",
		outPath,
	)
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("magick composite: %w\n%s", err, string(out))
	}
	return nil
}
