# Marketing Agency Example

This example implements a multi-agent marketing application in DMAS-Forge.

It is based on the `marketing-agency` example from Google's adk repo, but reimplemented in DMAS-Forge [original adk example](https://github.com/google/adk-samples/tree/a04174ca2904d073fc9a00640a87952f8d6491d1/python/agents/marketing-agency).

The coordinator orchestrates four specialized agents:

- `DomainAgent`: suggests candidate domains using DuckDuckGo search.
- `WebsiteAgent`: generates website files (`index.html`, `about.html`, `services.html`, `contact.html`, `style.css`, `script.js`).
- `MarketingAgent`: generates a full marketing strategy document.
- `LogoAgent`: generates a logo image via OpenAI images API and returns it inline as JPEG.

## Architecture

- Workflow code: `examples/marketing-agency/workflow`
- Wiring/deployment code: `examples/marketing-agency/wiring`

The workflow layer is protocol-agnostic. Wiring layer handles deployment as HTTP service in Docker.

# Differences from the ADK Reference Implementation

Unlike the original, where sub-agents are tools inside a single process, this reimplementation wires each agent as a separate Go service within the same process.

The original example returns raw text and does no parsing, but this version expects structured output and parses each agent's result with JSON deserialization first, falling back to code-block extraction, and ultimately returning the raw model output if neither works.

Small models like GPT-5.4-nano may struggle with website generation because the output is large and can get truncated. A stronger model like GPT-5.4 is more reliable.

# Limitations

- The code is meant for demonstration and experimentation purposes and isn't production-ready.
- As in the reference example, the coordinator calls agents in a fixed order, without dynamic delegation based on intermediate results.

## Setup

Edit `examples/marketing-agency/wiring/example_model.json`:

```json
{
  "name": "gpt-5.4-nano",
  "url": "https://api.openai.com/v1",
  "key": "your-api-key-here"
}
```

## Build and Run

```bash
cd examples/marketing-agency/wiring
go run main.go -w docker -o build -modfile=./example_model.json
cd build/docker
cp ../.local.env .env
docker compose build && docker compose up -d
```

## Usage

```bash
curl --get 'http://localhost:12345/CreateCampaign' \
  --data-urlencode 'req={
    "brand_name":"Organic Cakes Bakery",
    "keywords":["organic","cakes","bakery","artisan"],
    "description":"Artisan organic cake bakery in Zurich",
    "target_audience":"Health-conscious consumers aged 25-45 interested in organic products"
  }'
```

Expected response contains a `Ret0` object with:

- `domains`
- `selected_domain`
- `website_files`
- `marketing_strategy`
- `logo_jpeg` (base64-encoded JPEG image)
- `summary`
