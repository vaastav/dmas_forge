# RAG Chat Example

A single-agent chat application that showcases two different wiring configurations for the RAG agent, demonstrating different capabilities of the RAG plugin.

This example utilizes the **OpenAIKnowledgeBase** and **InMemoryVectorStore** features, with the an example of a few sourdough baking reference files in `workflow/data/*.md`. 

## Configurations

We're demonstrating two configurations:

- **`automatic`**: The retrieval process is `automatic`, where the RAG system gets pre-populated with documents and augments each incoming query with relevant chunks from these documents. The agent does not have any tools to review or modify the RAG system.

- **`agentic`**: The retrival process is entirely driven by the agent. The RAG system starts empty and exposes CRUD tools to the agent. The agent then has to search or modify the knowledge base as appropriate.

## Setup

Edit `wiring/example_model.json` with your OpenAI-compatible chat model, embeddings model, base URL, and API key:

```json
{
    "name": "gpt-5.4-nano",
    "url": "https://api.openai.com/v1",
    "key": "your-api-key-here",
    "embedding_model": "text-embedding-3-small"
}
```

## Build and Run

Replace `<MODE>` with `automatic` or `agentic` in the following:

```bash
cd examples/rag_chat/wiring
go run main.go -w <MODE> -o build -modfile=./example_model.json
cd build/docker
cp ../.local.env .env
docker compose build && docker compose up -d
```

## Usage

```bash
# Query about something in the knowledge base
curl -s --get --data-urlencode "message=What hydration range does the guide recommend for everyday sourdough?" \
  http://localhost:12345/Chat | jq -r .Ret0
  
# Ask the agent to add to the knowledge base (should fail in `automatic` mode)
curl -s --get --data-urlencode "message=Add a document to note that I like chocolate strawberry sourdough" \
  http://localhost:12345/Chat | jq -r .Ret0
  
# Query outside knowledge base scope
curl -s --get --data-urlencode "message=What is the capital of France?" \
  http://localhost:12345/Chat | jq -r .Ret0
```
