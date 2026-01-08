package vllm

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/address"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/service"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/plugins/docker"
	"github.com/blueprint-uservices/blueprint/plugins/golang/goparser"
	"github.com/blueprint-uservices/blueprint/plugins/workflow/workflowspec"
	"github.com/vaastav/agentic_blueprint/ai_runtime/plugins/openaiagent"
)

type VLLMContainer struct {
	docker.Container
	docker.ProvidesContainerInstance

	InstanceName string
	BindAddr     *address.BindConfig
	Iface        *goparser.ParsedInterface
	ModelName    string
	APIKey       string
	HF_TOKEN     string
}

type VLLMInterface struct {
	service.ServiceInterface
	Wrapped service.ServiceInterface
}

func (v *VLLMInterface) GetName() string {
	return "vllm(" + v.Wrapped.GetName() + ")"
}

func (v *VLLMInterface) GetMethods() []service.Method {
	return v.Wrapped.GetMethods()
}

func newVLLMContainer(name string, model_name string, apikey string, hf_token string) (*VLLMContainer, error) {
	spec, err := workflowspec.GetService[openaiagent.OpenAILLMClient]()
	if err != nil {
		return nil, err
	}

	proc := &VLLMContainer{
		InstanceName: name,
		Iface:        spec.Iface,
		ModelName:    model_name,
		APIKey:       apikey,
		HF_TOKEN:     hf_token,
	}
	return proc, nil
}

// Implements ir.IRNode
func (v *VLLMContainer) String() string {
	return v.InstanceName + " = vllmContainer(" + v.ModelName + "," + v.BindAddr.Name() + ")"
}

// Implements ir.IRNode
func (v *VLLMContainer) Name() string {
	return v.InstanceName
}

// Implements service.ServiceNode
func (node *VLLMContainer) GetInterface(ctx ir.BuildContext) (service.ServiceInterface, error) {
	iface := node.Iface.ServiceInterface(ctx)
	return &VLLMInterface{Wrapped: iface}, nil
}

// Implements docker.ProvidesContainerInstance
func (node *VLLMContainer) AddContainerInstance(target docker.ContainerWorkspace) error {
	instanceName := ir.CleanName(node.InstanceName)

	node.BindAddr.Hostname = instanceName
	node.BindAddr.Port = 8000

	err := target.DeclarePrebuiltInstance(node.InstanceName, "vllm/vllm-openai:latest", node.BindAddr)
	if err != nil {
		return err
	}

	// Add GPU support
	err = target.AddCustomConf(instanceName, "gpus", "all")
	if err != nil {
		return err
	}

	// Add volume for persisting modules
	err = target.AddVolume(instanceName, "~/.cache/huggingface", "/root/.cache/huggingface")
	if err != nil {
		return err
	}

	// Add custom command
	err = target.SetCustomCommand(instanceName, []string{"--model", node.ModelName, "--api-key", node.APIKey})
	if err != nil {
		return err
	}

	// Add HF_TOKEN to the environment
	return target.SetEnvironmentVariable(instanceName, "HUGGING_FACE_HUB_TOKEN", node.HF_TOKEN)
}
