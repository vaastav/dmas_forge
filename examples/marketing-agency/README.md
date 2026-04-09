# Marketing Agency Example

This example implements a multi-agent marketing application in DMAS-Forge.

The coordinator orchestrates four specialized agents:

- `DomainAgent`: suggests candidate domains using DuckDuckGo search.
- `WebsiteAgent`: generates website files (`index.html`, `about.html`, `services.html`, `contact.html`, `style.css`, `script.js`).
- `MarketingAgent`: generates a full marketing strategy document.
- `LogoAgent`: generates and saves a logo image via OpenAI images API.

## Architecture

- Workflow code: `examples/marketing-agency/workflow`
- Wiring/deployment code: `examples/marketing-agency/wiring`

The workflow layer is protocol-agnostic. Wiring layer handles deployment as HTTP service in Docker.

## Setup

Edit `examples/marketing-agency/wiring/example_model.json`:

```json
{
  "name": "gpt-4o-mini",
  "url": "https://api.openai.com/v1",
  "key": "your-api-key-here"
}
```

## Build and Run

```bash
cd examples/marketing-agency/wiring
go run main.go -w docker -o build -modfile=./example_model.json
cd build/docker
docker compose build && docker compose up -d
```

## Usage

```bash
curl -X POST 'http://localhost:12345/CreateCampaign' \
  -H 'Content-Type: application/json' \
  -d '{
    "brand_name": "Organic Cakes Bakery",
    "keywords": ["organic", "cakes", "bakery", "artisan"],
    "description": "Artisan organic cake bakery in Zurich",
    "target_audience": "Health-conscious consumers aged 25-45 interested in organic products"
  }'
```

Expected response contains:

- `domains`
- `selected_domain`
- `website_files`
- `marketing_strategy`
- `logo_filepath`
- `summary`

Generated images are written under `artifacts/` in the service working directory.
