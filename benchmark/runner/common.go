package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func repoPaths() (string, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	if filepath.Base(wd) == "benchmark" {
		return filepath.Dir(wd), wd, nil
	}
	return wd, filepath.Join(wd, "benchmark"), nil
}

func splitCSV(value string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out[part] = true
		}
	}
	return out
}

func latestRun(resultsRoot string) (string, error) {
	entries, err := os.ReadDir(resultsRoot)
	if err != nil {
		return "", err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no runs found under %s", resultsRoot)
	}
	sort.Strings(names)
	return names[len(names)-1], nil
}
