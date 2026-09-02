// Copyright 2026 dezeat
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

package pipeline

import (
	"encoding/json"
	"fmt"
	"testing"
)

// OpenAI's strict structured-output mode rejects any object schema that does
// not both pin "additionalProperties": false and list every property in
// "required". The adapter sends strict mode unconditionally, so a schema that
// bends these rules fails live with HTTP 400 while every fixture stays green —
// which is exactly how the live lane found it (#141).
func TestProviderFacingSchemasAreStrictModeCompliant(t *testing.T) {
	schemas := map[string]string{
		"candidateSchema":          candidateSchema,
		"verifierSchema":           verifierSchema,
		"critiqueSchema":           critiqueSchema,
		"ungroundedCritiqueSchema": ungroundedCritiqueSchema,
	}
	for name, raw := range schemas {
		var node map[string]any
		if err := json.Unmarshal([]byte(raw), &node); err != nil {
			t.Fatalf("%s does not parse as JSON: %v", name, err)
		}
		if err := checkStrictObject(node, name); err != nil {
			t.Errorf("%v", err)
		}
	}
}

func checkStrictObject(node map[string]any, path string) error {
	props, hasProps := node["properties"].(map[string]any)
	if hasProps {
		if ap, ok := node["additionalProperties"].(bool); !ok || ap {
			return fmt.Errorf("%s: object must pin \"additionalProperties\": false", path)
		}
		required, _ := node["required"].([]any)
		listed := make(map[string]bool, len(required))
		for _, r := range required {
			s, _ := r.(string)
			listed[s] = true
		}
		for key := range props {
			if !listed[key] {
				return fmt.Errorf("%s: property %q missing from \"required\"", path, key)
			}
			if child, ok := props[key].(map[string]any); ok {
				if err := checkStrictObject(child, path+"."+key); err != nil {
					return err
				}
			}
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		if err := checkStrictObject(items, path+".items"); err != nil {
			return err
		}
	}
	return nil
}
