# Marketing Agency Example

This example implements a multi-agent marketing application in DMAS-Forge.

It is based on the `marketing-agency` example from Google's adk repo, but reimplemented in DMAS-Forge [original adk example](https://github.com/google/adk-samples/tree/a04174ca2904d073fc9a00640a87952f8d6491d1/python/agents/marketing-agency).

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
- `logo_filepath`
- `summary`

Generated images are written under `artifacts/` in the service working directory.

Note: in this example, only the logo image is written to disk. Other campaign outputs
(`domains`, `website_files`, `marketing_strategy`, `summary`) are returned in the HTTP
response body and are not persisted by default.

## To Persist Artifacts in Docker

By default, `artifacts/` lives inside the container filesystem. To persist generated
logo files across container recreation, mount `/artifacts` to either a host directory
or a Docker named volume.

After generating the Docker wiring (`go run main.go -w docker -o build ...`), edit:

`examples/marketing-agency/wiring/build/docker/docker-compose.yml`

with a host bind mount

```yaml
services:
  coordinator_ctr:
    # ... existing fields
    volumes:
      - ./artifacts:/artifacts
```

or a docker named volume

```yaml
services:
  coordinator_ctr:
    # ... existing fields
    volumes:
      - campaign_artifacts:/artifacts

volumes:
  campaign_artifacts:
```
