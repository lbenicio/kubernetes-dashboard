// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package types

import (
	"encoding/json"
	"testing"
)

func TestCustomResourceObjectUnmarshal(t *testing.T) {
	var object CustomResourceObject
	if err := json.Unmarshal([]byte(`{"apiVersion":"example.test/v1","kind":"Widget","metadata":{"name":"sample","namespace":"team-a"}}`), &object); err != nil {
		t.Fatalf("unmarshal custom resource object: %v", err)
	}
	if object.TypeMeta.Kind != "Widget" || object.ObjectMeta.Name != "sample" || object.ObjectMeta.Namespace != "team-a" {
		t.Fatalf("custom resource object = %#v", object)
	}
	if err := json.Unmarshal([]byte(`{`), &object); err == nil {
		t.Fatal("CustomResourceObject.UnmarshalJSON() accepted invalid JSON")
	}
	if err := json.Unmarshal([]byte(`{"metadata":5}`), &object); err == nil {
		t.Fatal("CustomResourceObject.UnmarshalJSON() accepted invalid metadata")
	}
}

func TestCustomResourceObjectListUnmarshal(t *testing.T) {
	var list CustomResourceObjectList
	data := []byte(`{"apiVersion":"example.test/v1","kind":"WidgetList","items":[{"apiVersion":"example.test/v1","kind":"Widget","metadata":{"name":"sample"}}]}`)
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatalf("unmarshal custom resource object list: %v", err)
	}
	if list.TypeMeta.Kind != "WidgetList" || len(list.Items) != 1 || list.Items[0].ObjectMeta.Name != "sample" {
		t.Fatalf("custom resource object list = %#v", list)
	}
	if err := json.Unmarshal([]byte(`{`), &list); err == nil {
		t.Fatal("CustomResourceObjectList.UnmarshalJSON() accepted invalid JSON")
	}
	if err := json.Unmarshal([]byte(`{"items":{}}`), &list); err == nil {
		t.Fatal("CustomResourceObjectList.UnmarshalJSON() accepted a non-array items value")
	}
}
