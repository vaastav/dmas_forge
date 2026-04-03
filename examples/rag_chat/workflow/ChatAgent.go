package workflow

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

//go:embed all:data
var embeddedFS embed.FS

type ChatAgent interface {
	Chat(ctx context.Context, message string) (string, error)
}

type ChatAgentImpl struct {
	agent core.Agent
	kb    core.KnowledgeBase
}

func NewChatAgentImpl(ctx context.Context, agent core.Agent, kb core.KnowledgeBase, preIndexFiles string) (ChatAgent, error) {
	a := &ChatAgentImpl{
		agent: agent,
		kb:    kb,
	}

	prompt := "You are a friendly sourdough baking assistant. Answer questions using ONLY information from the knowledge base. If the knowledge base does not contain relevant information for a question, say \"I don't have information about that in my knowledge base\" rather than making up an answer."
	err := a.agent.AddSystemPrompt(ctx, prompt)
	if err != nil {
		return nil, err
	}

	err = loadDocuments(ctx, preIndexFiles, kb)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func loadDocuments(ctx context.Context, preIndexFiles string, kb core.KnowledgeBase) error {
	if preIndexFiles == "" {
		return nil
	}

	files, err := resolveFilesToLoad(preIndexFiles)
	if err != nil {
		return err
	}

	for _, name := range files {
		content, err := fs.ReadFile(embeddedFS, filepath.Join("data", name))
		if err != nil {
			return fmt.Errorf("read embedded file %s: %w", name, err)
		}
		docID := strings.TrimSuffix(name, filepath.Ext(name))
		if err := kb.Index(ctx, core.Document{
			ID:       docID,
			Content:  string(content),
			Metadata: map[string]any{"file_name": name},
		}); err != nil {
			return fmt.Errorf("index %s: %w", docID, err)
		}
	}
	return nil
}

func resolveFilesToLoad(preIndexFiles string) ([]string, error) {
	if preIndexFiles == "*" {
		entries, err := fs.ReadDir(embeddedFS, "data")
		if err != nil {
			return nil, fmt.Errorf("read embedded data dir: %w", err)
		}
		var files []string
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
				files = append(files, entry.Name())
			}
		}
		sort.Strings(files)
		return files, nil
	}

	var files []string
	for _, n := range strings.Split(preIndexFiles, ",") {
		trimmedName := strings.TrimSpace(n)
		if trimmedName == "" {
			continue
		}
		cleanName := filepath.Base(trimmedName)
		if cleanName != trimmedName || filepath.Ext(cleanName) != ".md" {
			return nil, fmt.Errorf("invalid knowledge file name: %s", trimmedName)
		}
		files = append(files, cleanName)
	}
	return files, nil
}

func (a *ChatAgentImpl) Chat(ctx context.Context, message string) (string, error) {
	return a.agent.LLMCallWithTools(ctx, message)
}
