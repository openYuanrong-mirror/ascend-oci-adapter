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
	"path/filepath"
	"testing"

	"ascend-common/api"
)

func TestAdapterLogPathMustBeAbsolute(t *testing.T) {
	t.Setenv(adapterLogPathEnv, filepath.Join("relative", "adapter.log"))
	if err := initMindClusterLogger(); err == nil {
		t.Fatal("relative adapter log path must be rejected")
	}
}

func TestProductSpecForDevType(t *testing.T) {
	tests := map[string]productSpec{
		api.Ascend310P:  {Generation: "310P", RuntimeFamily: "Ascend310P", ResourceFamily: "huawei.com/Ascend310P"},
		api.Ascend910B:  {Generation: "A2", RuntimeFamily: "Ascend910", ResourceFamily: "huawei.com/Ascend910"},
		api.Ascend910A3: {Generation: "A3", RuntimeFamily: "Ascend910", ResourceFamily: "huawei.com/Ascend910"},
	}
	for devType, expected := range tests {
		actual, err := productSpecForDevType(devType)
		if err != nil || actual != expected {
			t.Fatalf("product spec for %q = %#v, %v; want %#v", devType, actual, err, expected)
		}
	}
	if _, err := productSpecForDevType(api.Ascend910A); err == nil {
		t.Fatal("unsupported Ascend910A family must be rejected")
	}
}
