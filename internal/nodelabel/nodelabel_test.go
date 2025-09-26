/*
Copyright 2025.

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

package nodelabel

import (
	"context"
	"maps"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
)

func TestManagerGet(t *testing.T) {
	type testCase struct {
		name            string
		node            *v1.Node
		nodeName        string
		labelKey        string
		expectedValue   string
		expectedOK      bool
		expectedSuccess bool
	}

	testCases := []testCase{
		{
			name:            "no node",
			nodeName:        "test-node",
			labelKey:        "myCustomKey",
			expectedSuccess: false,
		},
		{
			name: "no labels",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
			},
			nodeName:        "test-node",
			labelKey:        "myCustomKey",
			expectedSuccess: true,
			expectedOK:      false,
		},
		{
			name: "missing label",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						"foo-label": "bar",
					},
				},
			},
			nodeName:        "test-node",
			labelKey:        "myCustomKey",
			expectedSuccess: true,
			expectedOK:      false,
		},
		{
			name: "found label",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						"myCustomKey": "VAL",
					},
				},
			},

			nodeName:        "test-node",
			labelKey:        "myCustomKey",
			expectedSuccess: true,
			expectedOK:      true,
			expectedValue:   "VAL",
		},
	}

	err := workshopv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Fatalf("cannot register to scheme: %v", err)
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			var cli client.Client
			if tcase.node != nil {
				cli = newFakeClient(tcase.node)
			} else {
				cli = newFakeClient()
			}
			mgr := NewManager(tcase.nodeName, cli)
			val, ok, err := mgr.Get(context.TODO(), tcase.labelKey)
			success := (err == nil)
			if success != tcase.expectedSuccess {
				t.Fatalf("unexpected status. wants=%v got=%v err=%v", tcase.expectedSuccess, success, err)
			}
			if ok != tcase.expectedOK {
				t.Errorf("ok got=%v expected=%v", ok, tcase.expectedOK)
			}
			if val != tcase.expectedValue {
				t.Errorf("value got=%v expected=%v", val, tcase.expectedValue)
			}
		})
	}
}

func TestManagerSet(t *testing.T) {
	type testCase struct {
		name       string
		node       *v1.Node
		nodeName   string
		labelKey   string
		labelValue string
		expectedOK bool
	}

	testCases := []testCase{
		{
			name: "bad label",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
			},
			nodeName:   "test-node",
			labelKey:   "myCustomKey",
			expectedOK: false,
		},
		{
			name: "unknown node",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
			},
			nodeName:   "unknown-unexpected-node",
			expectedOK: false,
		},
		{
			name: "set from empty",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
			},
			nodeName:   "test-node",
			labelKey:   ContentHash,
			labelValue: "test-fake-hash",
			expectedOK: true,
		},
		{
			name: "set from empty",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
			},
			nodeName:   "test-node",
			labelKey:   ContentHash,
			labelValue: "test-fake-hash",
			expectedOK: true,
		},
		{
			name: "overrides value",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						ContentHash: "test-fake-hash-old",
					},
				},
			},
			nodeName:   "test-node",
			labelKey:   ContentHash,
			labelValue: "test-fake-hash-new",
			expectedOK: true,
		},
	}

	err := workshopv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Fatalf("cannot register to scheme: %v", err)
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			cli := newFakeClient(tcase.node)
			mgr := NewManager(tcase.nodeName, cli)
			err := mgr.Set(context.TODO(), tcase.labelKey, tcase.labelValue)
			ok := (err == nil)
			if ok != tcase.expectedOK {
				t.Fatalf("unexpected status. err=%v", err)
			}
			if tcase.labelKey == "" {
				// nothing to check, let's cut it short
				return
			}
			var updatedNode v1.Node
			err = cli.Get(context.TODO(), client.ObjectKeyFromObject(tcase.node), &updatedNode)
			if err != nil {
				t.Fatalf("cannot get updated node %q: %v", tcase.node.Name, err)
			}
			updatedVal := updatedNode.Labels[tcase.labelKey]
			if updatedVal != tcase.labelValue {
				t.Fatalf("label %q mismatch: expected %q got %q", tcase.labelKey, tcase.labelValue, updatedVal)
			}
		})
	}
}

func TestManagerClear(t *testing.T) {
	type testCase struct {
		name       string
		node       *v1.Node
		nodeName   string
		expectedOK bool
	}

	testCases := []testCase{
		{
			name: "no label",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
			},
			nodeName:   "test-node",
			expectedOK: true,
		},
		{
			name: "unknown node",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
			},
			nodeName:   "unknown-unexpected-node",
			expectedOK: false,
		},
		{
			name: "labels set, but not the managed ones",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						"Foo": "quux",
						"Bar": "42",
					},
				},
			},
			nodeName:   "test-node",
			expectedOK: true,
		},
		{
			name: "removes value",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						ContentHash: "test-fake-hash",
						"foo":       "quux",
						"bar":       "42",
					},
				},
			},
			nodeName:   "test-node",
			expectedOK: true,
		},
	}

	err := workshopv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Fatalf("cannot register to scheme: %v", err)
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			cli := newFakeClient(tcase.node)
			mgr := NewManager(tcase.nodeName, cli)
			oldLabels := maps.Clone(tcase.node.Labels)
			err := mgr.Clear(context.TODO())
			ok := (err == nil)
			if ok != tcase.expectedOK {
				t.Fatalf("unexpected status. err=%v", err)
			}
			if !ok {
				if !maps.Equal(oldLabels, tcase.node.Labels) {
					t.Fatalf("mutated labels on error")
				}
				return
			}

			var updatedNode v1.Node
			err = cli.Get(context.TODO(), client.ObjectKeyFromObject(tcase.node), &updatedNode)
			if err != nil {
				t.Fatalf("cannot get updated node %q: %v", tcase.node.Name, err)
			}
			for key := range oldLabels {
				if IsValidKey(key) {
					if _, ok := updatedNode.Labels[key]; ok {
						t.Errorf("label %q not removed, but is managed, so it should be gone", key)
					}
				} else {
					if _, ok := updatedNode.Labels[key]; !ok {
						t.Errorf("label %q removed, but is not managed, so it should be kept", key)
					}
				}
			}
		})
	}
}

func newFakeClient(initObjects ...runtime.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithStatusSubresource(&workshopv1alpha1.Configuration{}).WithRuntimeObjects(initObjects...).Build()
}
