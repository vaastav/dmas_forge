package vllm

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/address"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/pointer"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
)

func VLLMAgent(spec wiring.WiringSpec, agent_name string, model_name string, apikey string, hf_token string) string {

	ctrName := model_name + ".vllm_ctr"
	addrName := model_name + ".addr"
	clientName := model_name + ".vllm_client"

	spec.Define(ctrName, &VLLMContainer{}, func(namespace wiring.Namespace) (ir.IRNode, error) {
		ctr, err := newVLLMContainer(ctrName, model_name, apikey, hf_token)
		if err != nil {
			return nil, err
		}
		err = address.Bind[*VLLMContainer](namespace, addrName, ctr, &ctr.BindAddr)
		return ctr, err
	})

	// Create a pointer to the vllm Container
	ptr := pointer.CreatePointer[*VLLMClient](spec, agent_name, ctrName)

	// Define the address that points to the vllm container
	address.Define[*VLLMContainer](spec, addrName, ctrName)

	// Add the address to the pointer
	ptr.AddAddrModifier(spec, addrName)

	clientNext := ptr.AddSrcModifier(spec, clientName)
	spec.Define(clientName, &VLLMClient{}, func(namespace wiring.Namespace) (ir.IRNode, error) {
		addr, err := address.Dial[*VLLMContainer](namespace, clientNext)
		if err != nil {
			return nil, err
		}
		return newVLLMClient(clientName, addr.Dial, model_name, apikey)
	})

	return agent_name
}
