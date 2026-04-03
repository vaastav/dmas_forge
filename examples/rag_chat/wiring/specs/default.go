package specs

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

type ModelInfo struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Key            string `json:"key"`
	EmbeddingModel string `json:"embedding_model"`
}

var modelFile = flag.String("modfile", "model.json", "Specific model related information")

func readModelInfo() (ModelInfo, error) {
	var model ModelInfo
	file, err := os.Open(*modelFile)
	if err != nil {
		return ModelInfo{}, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return ModelInfo{}, err
	}
	if err := json.Unmarshal(bytes, &model); err != nil {
		return ModelInfo{}, err
	}
	if strings.TrimSpace(model.EmbeddingModel) == "" {
		return ModelInfo{}, fmt.Errorf("embedding_model must be set in %s", *modelFile)
	}
	return model, nil
}
