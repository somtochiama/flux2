/*
Copyright 2021 The Flux authors

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

package main

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
)

// notificationv1.Receiver

var receiverType = apiType{
	kind:         notificationv1.ReceiverKind,
	humanKind:    "receiver",
	groupVersion: notificationv1.GroupVersion,
}

type receiverAdapter struct {
	*notificationv1.Receiver
}

func (a receiverAdapter) asClientObject() client.Object {
	return a.Receiver
}

func (a receiverAdapter) deepCopyClientObject() client.Object {
	return a.Receiver.DeepCopy()
}

// notificationv1.Receiver

type receiverListAdapter struct {
	*notificationv1.ReceiverList
}

func (a receiverListAdapter) asClientList() client.ObjectList {
	return a.ReceiverList
}

func (a receiverListAdapter) len() int {
	return len(a.ReceiverList.Items)
}
