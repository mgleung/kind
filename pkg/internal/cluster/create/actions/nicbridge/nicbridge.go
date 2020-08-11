/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package kubeadminit implements the kubeadm init action
package nicbridge

import (
	"fmt"

	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/internal/cluster/create/actions"
)

// kubeadmInitAction implements action for executing the kubadm init
// and a set of default post init operations like e.g. install the
// CNI network plugin.
type action struct{}

// NewAction returns a new action for kubeadm init
func NewAction() actions.Action {
	return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
	fmt.Print("RUNNING THE BRIDGE CREATION ACTION\n")
	allNodes, err := ctx.Nodes()
	if err != nil {
		return err
	}

	for _, node := range allNodes {
		// Add the bridge interface for the node
		fmt.Printf("Add the bridge br0 for node: %v\n", node)
		cmd := node.Command("ip", "link", "add", "name", "br0", "type", "bridge")
		err := cmd.Run()
		if err != nil {
			return errors.Wrap(err, "failed to create the bridge")
		}

		// Bring up the bridge
		fmt.Printf("Activate the bridge br0 for node: %v\n", node)
		cmd = node.Command("ip", "link", "set", "dev", "br0", "up")
		err = cmd.Run()
		if err != nil {
			return errors.Wrap(err, "failed to bring up the bridge")
		}

		// Assign an IP address to the bridge
		// TODO: Make the address configurable since this will probably differ per different nodes.
		bridgeAddr := "172.31.13.1"
		fmt.Printf("Assigning address %v to br0 on node: %v\n", bridgeAddr, node)
		cmd = node.Command("ip", "addr", "add", bridgeAddr+"/24", "dev", "br0")
		err = cmd.Run()
		if err != nil {
			return errors.Wrap(err, "failed to assign address to the bridge")
		}

		// Delete the default route
		fmt.Print("Deleting the default route to replace it with the bridge\n")
		cmd = node.Command("ip", "route", "delete", "default")
		err = cmd.Run()
		if err != nil {
			return errors.Wrap(err, "failed to delete the default route")
		}

		// Set the bridge as the default route
		fmt.Print("Assigning the bridge as the default route\n")
		cmd = node.Command("ip", "route", "add", "default", "via", bridgeAddr)
		err = cmd.Run()
		if err != nil {
			return errors.Wrap(err, "failed to assign default route to bridge")
		}

		nics, err := node.NICs()
		if err != nil {
			fmt.Printf("NIC action error: %v\n", err)
			continue
		}

		// Attach the network interfaces to the bridge.
		for _, nic := range nics {
			fmt.Printf("Add bridge br0 for interface %v on node %v\n", nic, node)
			cmd := node.Command(
				"ip",
				"link",
				"set",
				"dev",
				nic,
				"master",
				"br0",
			)
			err := cmd.Run()
			if err != nil {
				return errors.Wrap(err, "failed to connect interface to bridge")
			}
		}
	}
	return nil
}
