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

import "testing"

func TestProviderVersion(t *testing.T) {
	original := adapterVersion
	t.Cleanup(func() { adapterVersion = original })
	adapterVersion = "v0.1.0"
	if actual, expected := providerVersion(), "v0.1.0+mindcluster-v26.2.0.beta.1"; actual != expected {
		t.Fatalf("provider version = %q; want %q", actual, expected)
	}
}

func TestNormalizeProductModel(t *testing.T) {
	tests := map[string]string{
		"310P3":          "ascend310p3",
		"910B4":          "ascend910b4",
		"Ascend910B2C":   "ascend910b2c",
		"Ascend910_9391": "ascend910_9391",
		"Ascend910":      "ascend910",
		" Acme NPU-X ":   "ascendacmenpu-x",
	}
	for raw, expected := range tests {
		model, err := normalizeProductModel(raw)
		if err != nil || model != expected {
			t.Fatalf("normalize %q = %q, %v; want %q", raw, model, err, expected)
		}
	}
	for _, raw := range []string{"", "  ", "!!!"} {
		if _, err := normalizeProductModel(raw); err == nil {
			t.Fatalf("empty normalized model %q must be rejected", raw)
		}
	}
}
