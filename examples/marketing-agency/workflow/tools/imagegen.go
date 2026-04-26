package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"image/png"
	"strings"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

// ImageGenTool returns the OpenAI function-calling tool definition for
// generate_image.  The LLM supplies a prompt and receives a short status
// confirmation; the actual image bytes are captured out-of-band via a
// per-request buffer read by ImageGenHandler from the call context.
func ImageGenTool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        "generate_image",
			Description: openai.String("Generate a logo image from a prompt."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "Image prompt",
					},
				},
				"required": []string{"prompt"},
			},
		},
	}
}

// ImageGenHandler returns a tool handler that calls the DALL-E API,
// converts the resulting PNG to JPEG, and stores the compressed bytes in
// a per-request buffer found on the context. The string returned to the
// LLM is a brief status message — the image data never flows through the
// LLM context.
func ImageGenHandler(client *openai.Client) core.ToolHandlerFn {
	return func(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
		if tc.Function.Name != "generate_image" {
			return "", fmt.Errorf("unsupported tool: %s", tc.Function.Name)
		}

		var args struct {
			Prompt string `json:"prompt"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		if strings.TrimSpace(args.Prompt) == "" {
			return "", fmt.Errorf("empty prompt")
		}

		jpegBytes, err := generateJPEG(ctx, client, args.Prompt)
		if err != nil {
			return "", err
		}

		dst, err := imageOutputFromContext(ctx)
		if err != nil {
			return "", err
		}
		*dst = jpegBytes

		return `{"status":"success"}`, nil
	}
}

// generateJPEG calls the DALL-E 3 images API with b64_json response
// format, decodes the PNG payload, and re-encodes it as JPEG (quality 85).
func generateJPEG(ctx context.Context, client *openai.Client, prompt string) ([]byte, error) {
	resp, err := client.Images.Generate(ctx, openai.ImageGenerateParams{
		Model:          openai.ImageModelDallE3,
		Prompt:         prompt,
		N:              openai.Int(1),
		Quality:        openai.ImageGenerateParamsQualityStandard,
		Size:           openai.ImageGenerateParamsSize1024x1024,
		ResponseFormat: openai.ImageGenerateParamsResponseFormatB64JSON,
	})
	if err != nil {
		return nil, fmt.Errorf("generate image: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("empty image response")
	}

	raw, err := base64.StdEncoding.DecodeString(resp.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("decode base64 payload: %w", err)
	}

	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode png: %w", err)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}

	return buf.Bytes(), nil
}
