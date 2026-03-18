package workflow

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

//go:embed all:data
var embeddedFS embed.FS

type ChatAgent interface {
	Chat(ctx context.Context, message string) (string, error)
}

type ChatAgentImpl struct {
	agent          core.Agent
	kb             core.KnowledgeBase
	enableFileTool bool
}

var fileToolDefs = map[string]openai.ChatCompletionToolParam{
	"list_knowledge_files": {
		Function: openai.FunctionDefinitionParam{
			Name:        "list_knowledge_files",
			Description: openai.String("List markdown files that can be read and indexed into the knowledge base."),
			Parameters: openai.FunctionParameters{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	},
	"read_knowledge_file": {
		Function: openai.FunctionDefinitionParam{
			Name:        "read_knowledge_file",
			Description: openai.String("Read the contents of a markdown knowledge file by name so it can be summarized or indexed."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"file_name": map[string]string{
						"type":        "string",
						"description": "The markdown file to read, such as 'troubleshooting.md'",
					},
				},
				"required": []string{"file_name"},
			},
		},
	},
}

func NewChatAgentImpl(ctx context.Context, agent core.Agent, kb core.KnowledgeBase, startupIndex string, enableFileTool string, preIndexFiles string) (ChatAgent, error) {
	startupIndexEnabled, err := strconv.ParseBool(startupIndex)
	if err != nil {
		return nil, fmt.Errorf("parse startupIndex: %w", err)
	}
	fileToolEnabled, err := strconv.ParseBool(enableFileTool)
	if err != nil {
		return nil, fmt.Errorf("parse enableFileTool: %w", err)
	}

	a := &ChatAgentImpl{
		agent:          agent,
		kb:             kb,
		enableFileTool: fileToolEnabled,
	}

	prompt := "You are a friendly sourdough baking assistant. Answer questions using ONLY information from the knowledge base. If the knowledge base does not contain relevant information for a question, say \"I don't have information about that in my knowledge base\" rather than making up an answer."
	if fileToolEnabled {
		prompt += " If needed, first call `list_knowledge_files`, then `read_knowledge_file`, and then use `index_document` to add a useful file to the knowledge base before answering."
	}
	if err := a.agent.AddSystemPrompt(ctx, prompt); err != nil {
		return nil, err
	}

	if fileToolEnabled {
		if err := a.agent.AddTools(ctx, fileToolDefs); err != nil {
			return nil, err
		}
		if err := a.agent.RegisterToolCallHandler(ctx, a.toolHandler); err != nil {
			return nil, err
		}
	}

	if strings.TrimSpace(preIndexFiles) != "" {
		docs, err := loadDocumentsByName(strings.Split(preIndexFiles, ","))
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			if err := kb.Index(ctx, doc); err != nil {
				return nil, fmt.Errorf("index %s: %w", doc.ID, err)
			}
		}
	} else if startupIndexEnabled {
		docs, err := loadDocumentsFromDir()
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			if err := kb.Index(ctx, doc); err != nil {
				return nil, fmt.Errorf("index %s: %w", doc.ID, err)
			}
		}
	}

	return a, nil
}

func loadDocumentsByName(names []string) ([]core.Document, error) {
	docs := make([]core.Document, 0, len(names))
	for _, name := range names {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}
		content, err := readKnowledgeFile(trimmedName)
		if err != nil {
			return nil, err
		}
		docID := strings.TrimSuffix(trimmedName, filepath.Ext(trimmedName))
		docs = append(docs, core.Document{
			ID:      docID,
			Content: content,
			Metadata: map[string]any{
				"file_name": trimmedName,
			},
		})
	}
	return docs, nil
}

func (a *ChatAgentImpl) Chat(ctx context.Context, message string) (string, error) {
	return a.agent.LLMCallWithTools(ctx, message)
}

func (a *ChatAgentImpl) toolHandler(ctx context.Context, toolCall openai.ChatCompletionMessageToolCall) (string, error) {
	switch toolCall.Function.Name {
	case "list_knowledge_files":
		files, err := listKnowledgeFiles()
		if err != nil {
			return "", err
		}
		if len(files) == 0 {
			return "No knowledge files available.", nil
		}
		return "Available knowledge files: " + strings.Join(files, ", "), nil
	case "read_knowledge_file":
		var args struct {
			FileName string `json:"file_name"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return "", fmt.Errorf("read_knowledge_file: invalid arguments: %w", err)
		}
		content, err := readKnowledgeFile(args.FileName)
		if err != nil {
			return "", err
		}
		return content, nil
	default:
		return "", fmt.Errorf("unsupported tool call: %s", toolCall.Function.Name)
	}
}

func loadDocumentsFromDir() ([]core.Document, error) {
	files, err := listKnowledgeFiles()
	if err != nil {
		return nil, err
	}

	docs := make([]core.Document, 0, len(files))
	for _, name := range files {
		content, err := readKnowledgeFile(name)
		if err != nil {
			return nil, err
		}
		docID := strings.TrimSuffix(name, filepath.Ext(name))
		docs = append(docs, core.Document{
			ID:      docID,
			Content: content,
			Metadata: map[string]any{
				"file_name": name,
			},
		})
	}
	return docs, nil
}

func listKnowledgeFiles() ([]string, error) {
	entries, err := fs.ReadDir(embeddedFS, "data")
	if err != nil {
		return nil, fmt.Errorf("read embedded data dir: %w", err)
	}

	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	return files, nil
}

func readKnowledgeFile(fileName string) (string, error) {
	cleanName := filepath.Base(fileName)
	if cleanName != fileName || filepath.Ext(cleanName) != ".md" {
		return "", fmt.Errorf("invalid knowledge file name: %s", fileName)
	}

	content, err := fs.ReadFile(embeddedFS, filepath.Join("data", cleanName))
	if err != nil {
		return "", fmt.Errorf("read embedded knowledge file %s: %w", cleanName, err)
	}
	return string(content), nil
}
