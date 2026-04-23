package model

import (
	"encoding/json"
	"flag"
	"io"
	"os"
)

type ModelInfo struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Key            string `json:"key"`
	EmbeddingModel string `json:"embedding_model"`
}

var model_file = flag.String("modfile", "model.json", "Specific model related information")

func GetModelInfo() (ModelInfo, error) {
	var minfo ModelInfo
	file, err := os.Open(*model_file)
	if err != nil {
		return minfo, err
	}
	defer file.Close()

	all_bytes, err := io.ReadAll(file)
	if err != nil {
		return minfo, err
	}
	err = json.Unmarshal(all_bytes, &minfo)
	if err != nil {
		return minfo, err
	}

	return minfo, nil
}
