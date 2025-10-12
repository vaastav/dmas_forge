# DMAS-Forge: A Framework for Transparent Deployment of AI Applications as Distributed Systems

Current Status and Features:

Agentic AI plugins:
+ openAI Agents: [ai_plugins/openai_plugin](ai_plugins/openai_plugin)
+ A2A support: [ai_plugins/a2a](ai_plugins/a2a)
+ MCP support: Coming Soon
+ vLLM support: Coming Soon
+ kagent support: Coming Soon

Blueprint plugins:
+ Blueprint: [Plugins](https://github.com/Blueprint-uServices/blueprint/tree/main/plugins)

## How to Cite?

```
@inproceedings{cornacchia2025dmasforge,
  title        = {DMAS-Forge: A Framework for Transparent Deployment of AI Applications as Distributed Systems},
  author       = {Cornacchia, Alessandro and Anand, Vaastav and Bilal, Muhammad and Qazi, Zafar and Canini, Marco},
  booktitle    = {1st Workshop on Systems for Agentic AI (SAA '25)},
  year         = {2025},
  keywords     = {distributed systems, AI applications, framework, deployment},
}
```

## Example Application

Currently, we have 1 simple application consisting of 2 agents (1 with a tool and 1 without).

### Setting up the model-apis

Edit the ```examples/weather/wiring/example_model.json` file to add your own API key, model name, and URL. The example assumes you are using chatgpt and model gpt-3.5-turbo. If you are using that model then simply replace the key field with the api key.

### Running the Weather application

```bash
cd examples/weather/wiring
go run main.go -w docker -o build -modfile=./example_model.json
cd build/docker
cp ../.local.env .env
docker compose build && docker compose up -d
```

### Testing the weather application w/ HTTP

```bash
curl 'http://localhost:12346/Query?query=London,England%20Weather'
```

You should an output that looks pretty much like the following (the output is mostly deterministic because of the tool call):

```bash
{"Ret0":"The current weather in London, England is 30 degrees Celsius.\nThere is not enough information available to determine the likelihood of a natural disaster based solely on the temperature in London, England."}
```