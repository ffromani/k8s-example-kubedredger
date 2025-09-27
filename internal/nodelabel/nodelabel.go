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
	"errors"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ContentHashV1 represent the hash value of the configuration value
	// You can only compare this value for equality.
	ContentHash = "workshop.golab.io/contentHashV1"
)

var (
	// ErrUnknownKey is returned if a key is not supported by the Manager
	ErrUnknownKey = errors.New("unsupported key")
)

// Manager handles the configuration labels on Kubernetes node objects
type Manager struct {
	nodeName string
	cli      client.Client
}

// IsValidKey returns true if the given key can be handled by the Manager, false otherwise
func IsValidKey(key string) bool {
	return key == ContentHash // for now only one key supported
}

// NewManager creates a new manager instance for the given node.
func NewManager(nodeName string, cli client.Client) *Manager {
	return &Manager{
		nodeName: nodeName,
		cli:      cli,
	}
}

// Set adds the given key with the given label to the labels of the node
// handled by this Manager.
func (mgr *Manager) Set(ctx context.Context, key, value string) error {
	if !IsValidKey(key) {
		return ErrUnknownKey
	}
	node := v1.Node{}
	err := mgr.cli.Get(ctx, client.ObjectKey{Name: mgr.nodeName}, &node)
	if err != nil {
		return err
	}
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	node.Labels[key] = value
	return mgr.cli.Update(ctx, &node)
}

// Get retrieves the value for the given key among the labels of the node
// handled by this Manager. Returns the value of the label, a boolean
// which is true if the label was found. If the boolean is false, the value
// has no meaning and must be ignored; if the operation failes, error is not nil
// and all the other returned values have no meaning.
func (mgr *Manager) Get(ctx context.Context, key string) (string, bool, error) {
	node := v1.Node{}
	err := mgr.cli.Get(ctx, client.ObjectKey{Name: mgr.nodeName}, &node)
	if err != nil {
		return "", false, err
	}
	if node.Labels == nil {
		return "", false, nil
	}
	value, ok := node.Labels[key]
	return value, ok, nil
}

// Clear removes all the labels known by this manager from
// the managed node. If a label is managed but never added,
// it is skipped silently.
func (mgr *Manager) Clear(ctx context.Context) error {
	node := v1.Node{}
	err := mgr.cli.Get(ctx, client.ObjectKey{Name: mgr.nodeName}, &node)
	if err != nil {
		return err
	}
	if node.Labels == nil {
		return nil // nothing to do
	}
	delete(node.Labels, ContentHash)
	return mgr.cli.Update(ctx, &node)
}
