package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

func ImageGenTool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        "generate_image",
			Description: openai.String("Generate a logo image and save it to local artifacts."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "Image prompt",
					},
					"filename": map[string]interface{}{
						"type":        "string",
						"description": "Filename stem without extension",
					},
				},
				"required": []string{"prompt", "filename"},
			},
		},
	}
}

func ImageGenHandler(client *openai.Client, outputDir string) core.ToolHandlerFn {
	return func(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
		if tc.Function.Name != "generate_image" {
			return "", fmt.Errorf("unsupported tool: %s", tc.Function.Name)
		}

		var args struct {
			Prompt   string `json:"prompt"`
			Filename string `json:"filename"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if strings.TrimSpace(args.Filename) == "" {
			args.Filename = "logo_" + fmt.Sprint(time.Now().Unix())
		}

		filePath, err := generateAndSave(ctx, client, outputDir, args.Prompt, args.Filename)
		if err != nil {
			return "", err
		}

		payload := map[string]string{
			"status":   "success",
			"filepath": filePath,
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func generateAndSave(ctx context.Context, client *openai.Client, outputDir, prompt, filename string) (string, error) {
	resp, err := client.Images.Generate(ctx, openai.ImageGenerateParams{
		Model:          openai.ImageModelDallE3,
		Prompt:         prompt,
		N:              openai.Int(1),
		Quality:        openai.ImageGenerateParamsQualityStandard,
		Size:           openai.ImageGenerateParamsSize1024x1024,
		ResponseFormat: openai.ImageGenerateParamsResponseFormatURL,
	})
	if err != nil {
		return "", fmt.Errorf("generate image: %w", err)
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("empty image response")
	}

	artifactsDir := filepath.Join(outputDir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return "", fmt.Errorf("create artifacts dir: %w", err)
	}

	safe := sanitizeFilename(filename)
	target := filepath.Join(artifactsDir, safe+".png")

	if strings.TrimSpace(resp.Data[0].URL) != "" {
		if err := downloadToFile(ctx, resp.Data[0].URL, target); err != nil {
			return "", fmt.Errorf("download image: %w", err)
		}
		return target, nil
	}

	if strings.TrimSpace(resp.Data[0].B64JSON) != "" {
		raw, err := base64.StdEncoding.DecodeString(resp.Data[0].B64JSON)
		if err != nil {
			return "", fmt.Errorf("decode image bytes: %w", err)
		}
		if err := os.WriteFile(target, raw, 0o644); err != nil {
			return "", fmt.Errorf("write image: %w", err)
		}
		return target, nil
	}

	return "", fmt.Errorf("image response contained neither URL nor b64 payload")
}

func downloadToFile(ctx context.Context, imageURL, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return os.WriteFile(target, b, 0o644)
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return "logo"
	}
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		if r == ' ' {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "logo"
	}
	return b.String()
}
