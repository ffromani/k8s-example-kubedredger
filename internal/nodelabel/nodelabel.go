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
	ContentHash = "workshop.golab.io/contentHash"
)

var (
	ErrUnknownKey = errors.New("unsupported key")
)

type Manager struct {
	nodeName string
	cli      client.Client
}

func IsValidKey(key string) bool {
	return key == ContentHash // for now only one key supported
}

func NewManager(nodeName string, cli client.Client) *Manager {
	return &Manager{
		nodeName: nodeName,
		cli:      cli,
	}
}

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
