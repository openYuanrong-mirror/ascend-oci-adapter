//go:build linux

// Copyright (c) 2026 Ant Group Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ascend-common/api"
	"ascend-common/cdi"
	cdimount "ascend-common/cdi/mount"
	"ascend-common/common-utils/hwlog"
	"ascend-common/devmanager"
	"ascend-common/devmanager/dcmi"
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

const maxInputBytes = 1 << 20

const (
	adapterLogPathEnv     = "ASCEND_OCI_ADAPTER_LOG_PATH"
	defaultAdapterLogPath = "/var/log/ascend-oci-adapter/ascend-oci-adapter.log"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("operation is required")
	}
	encoder := json.NewEncoder(stdout)
	switch args[0] {
	case "version":
		return encoder.Encode(versionResponse{SchemaVersion: schemaVersion, ProviderVersion: providerVersion()})
	case "discover":
		if err := initMindClusterLogger(); err != nil {
			return err
		}
		response, err := discover()
		if err != nil {
			return err
		}
		return encoder.Encode(response)
	case "edits":
		limited := io.LimitReader(stdin, maxInputBytes+1)
		raw, err := io.ReadAll(limited)
		if err != nil {
			return err
		}
		if len(raw) > maxInputBytes {
			return errors.New("edits input exceeds size limit")
		}
		var request editsRequest
		if err := json.Unmarshal(raw, &request); err != nil {
			return fmt.Errorf("parse edits request: %w", err)
		}
		if err := initMindClusterLogger(); err != nil {
			return err
		}
		response, err := buildEdits(request)
		if err != nil {
			return err
		}
		return encoder.Encode(response)
	default:
		return fmt.Errorf("unsupported operation %q", args[0])
	}
}

func initMindClusterLogger() error {
	logPath := strings.TrimSpace(os.Getenv(adapterLogPathEnv))
	if logPath == "" {
		logPath = defaultAdapterLogPath
	}
	if !filepath.IsAbs(logPath) {
		return fmt.Errorf("%s must be an absolute path", adapterLogPathEnv)
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err != nil {
		return fmt.Errorf("create adapter log directory: %w", err)
	}
	config := &hwlog.LogConfig{
		LogFileName: logPath,
		OnlyToFile:  true,
		MaxBackups:  hwlog.DefaultBackups,
		MaxAge:      hwlog.DefaultMinSaveAge,
	}
	if err := hwlog.InitRunLogger(config, context.Background()); err != nil {
		return fmt.Errorf("initialize MindCluster logger: %w", err)
	}
	return nil
}

func discover() (discoveryResponse, error) {
	manager, err := devmanager.AutoInit("", 30)
	if err != nil {
		return discoveryResponse{}, err
	}
	defer manager.ShutDown() //nolint:errcheck
	product, err := productSpecForDevType(manager.GetDevType())
	if err != nil {
		return discoveryResponse{}, err
	}
	_, logicIDs, err := manager.GetDeviceList()
	if err != nil {
		return discoveryResponse{}, err
	}
	devices := make([]device, 0, len(logicIDs))
	for _, logicID := range logicIDs {
		physicalID, err := manager.GetPhysicIDFromLogicID(logicID)
		if err != nil {
			return discoveryResponse{}, err
		}
		chip, err := manager.GetChipInfo(logicID)
		if err != nil {
			return discoveryResponse{}, err
		}
		rawProduct := chip.Name
		if rawProduct == "" {
			rawProduct = chip.Type
		}
		productModel, err := normalizeProductModel(rawProduct)
		if err != nil {
			return discoveryResponse{}, err
		}
		stableID, err := manager.GetDieID(logicID, dcmi.VDIE)
		if err != nil || strings.TrimSpace(stableID) == "" {
			return discoveryResponse{}, fmt.Errorf("get stable VDie ID for logic ID %d: %w", logicID, err)
		}
		health, healthErr := manager.GetDeviceHealth(logicID)
		devices = append(devices, device{
			SchedulerID: uint32(logicID), LogicID: logicID, PhysicalID: physicalID,
			// MindCluster's whole-device allocation uses the physical ID for these supported families.
			DeviceNodeID: physicalID,
			StableID:     stableID, ProductModel: productModel, Generation: product.Generation,
			RuntimeFamily: product.RuntimeFamily, ResourceFamily: product.ResourceFamily,
			RawProduct: rawProduct, Healthy: healthErr == nil && health == 0,
		})
	}
	return discoveryResponse{SchemaVersion: schemaVersion, ProviderVersion: providerVersion(), Devices: devices}, nil
}

func buildEdits(request editsRequest) (editsResponse, error) {
	if request.SchemaVersion != schemaVersion || !request.PhysicalOnly ||
		!supportedRuntimeFamily(request.RuntimeFamily) || len(request.LogicIDs) == 0 {
		return editsResponse{}, errors.New("invalid physical Ascend edits request")
	}
	nodeIDs := request.LogicIDs
	if len(request.DeviceNodeIDs) != 0 {
		if len(request.DeviceNodeIDs) != len(request.LogicIDs) {
			return editsResponse{}, errors.New("invalid Ascend device node IDs")
		}
		nodeIDs = request.DeviceNodeIDs
	}
	deviceIDs := make([]int, len(nodeIDs))
	seenNodes := make(map[int32]struct{}, len(nodeIDs))
	seen := make(map[int32]struct{}, len(request.LogicIDs))
	for index, logicID := range request.LogicIDs {
		if logicID < 0 {
			return editsResponse{}, fmt.Errorf("invalid logic ID %d", logicID)
		}
		if _, duplicate := seen[logicID]; duplicate {
			return editsResponse{}, fmt.Errorf("duplicate logic ID %d", logicID)
		}
		seen[logicID] = struct{}{}
		nodeID := nodeIDs[index]
		if nodeID < 0 {
			return editsResponse{}, fmt.Errorf("invalid device node ID %d", nodeID)
		}
		if _, duplicate := seenNodes[nodeID]; duplicate {
			return editsResponse{}, fmt.Errorf("duplicate device node ID %d", nodeID)
		}
		seenNodes[nodeID] = struct{}{}
		deviceIDs[index] = int(nodeID)
	}
	if filepath.Base(request.MountProfile) != "mounts.json" {
		return editsResponse{}, errors.New("mount profile must name mounts.json")
	}
	spec, err := cdi.BuildSpec(cdi.BuildSpecConfig{
		DeviceConfig: cdi.DeviceConfig{DeviceIDs: deviceIDs, DevType: request.RuntimeFamily, UseVirtual: false},
		MountConfig:  cdimount.MountConfig{Dir: filepath.Dir(request.MountProfile), MountUBDrv: false, AllowLink: false},
	})
	if err != nil {
		return editsResponse{}, err
	}
	return flattenSpec(spec, request.LogicIDs, nodeIDs)
}

// supportedProducts is the admission policy for physical devices. OCI generation
// uses the discovered node IDs and delegates variant-specific edits to CDI.
var supportedProducts = map[string]productSpec{
	api.Ascend310P:  {Generation: "310P", RuntimeFamily: "Ascend310P", ResourceFamily: "huawei.com/Ascend310P"},
	api.Ascend910B:  {Generation: "A2", RuntimeFamily: "Ascend910", ResourceFamily: "huawei.com/Ascend910"},
	api.Ascend910A3: {Generation: "A3", RuntimeFamily: "Ascend910", ResourceFamily: "huawei.com/Ascend910"},
}

func supportedRuntimeFamily(runtimeFamily string) bool {
	for _, product := range supportedProducts {
		if product.RuntimeFamily == runtimeFamily {
			return true
		}
	}
	return false
}

func productSpecForDevType(devType string) (productSpec, error) {
	if product, ok := supportedProducts[devType]; ok {
		return product, nil
	}
	return productSpec{}, fmt.Errorf("unsupported Ascend device family %q", devType)
}

func flattenSpec(spec *cdispec.Spec, logicIDs, nodeIDs []int32) (editsResponse, error) {
	response := editsResponse{
		SchemaVersion: schemaVersion, ProviderVersion: providerVersion(),
		Env: make(map[string]string),
	}
	byName := make(map[string]cdispec.ContainerEdits, len(spec.Devices))
	for _, current := range spec.Devices {
		byName[current.Name] = current.ContainerEdits
	}
	for _, nodeID := range nodeIDs {
		edits, ok := byName[strconv.Itoa(int(nodeID))]
		if !ok {
			return editsResponse{}, fmt.Errorf("CDI spec is missing device node ID %d", nodeID)
		}
		for _, node := range edits.DeviceNodes {
			response.Devices = append(response.Devices, flattenDevice(node))
		}
	}
	for _, node := range spec.ContainerEdits.DeviceNodes {
		if strings.HasPrefix(node.Path, "/dev/uburma/") || strings.HasPrefix(node.Path, "/dev/ummu/") {
			continue
		}
		response.SharedDevices = append(response.SharedDevices, flattenDevice(node))
	}
	for _, mount := range spec.ContainerEdits.Mounts {
		response.Mounts = append(response.Mounts, mountEdit{
			Source: mount.HostPath, Destination: mount.ContainerPath,
			Type: "bind", Options: append([]string(nil), mount.Options...),
		})
	}
	for _, env := range spec.ContainerEdits.Env {
		key, value, found := strings.Cut(env, "=")
		if !found || key == "" {
			return editsResponse{}, fmt.Errorf("invalid CDI environment %q", env)
		}
		response.Env[key] = value
	}
	visible := make([]string, len(logicIDs))
	for index, logicID := range logicIDs {
		visible[index] = strconv.Itoa(int(logicID))
	}
	response.Env["ASCEND_VISIBLE_DEVICES"] = strings.Join(visible, ",")
	response.Env["ASCEND_RT_VISIBLE_DEVICES"] = strings.Join(visible, ",")
	return response, nil
}

func flattenDevice(node *cdispec.DeviceNode) deviceEdit {
	return deviceEdit{
		HostPath: node.HostPath, ContainerPath: node.Path, Type: node.Type,
		Major: node.Major, Minor: node.Minor, Permissions: "rwm",
	}
}
