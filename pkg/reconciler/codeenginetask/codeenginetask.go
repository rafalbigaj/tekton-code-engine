/*
Copyright 2020 The Knative Authors

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

package codeenginetask

import (
	"context"
	"time"

	"github.com/rafalbigaj/tekton-code-engine/pkg/apis/codeenginetask"
	codeenginetaskv1alpha1 "github.com/rafalbigaj/tekton-code-engine/pkg/apis/codeenginetask/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	listersalpha "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

// podOwnerLabelKey is the key to a label that points to the owner (creator) of the
// pod, allowing us to easily list all pods a single CodeEngineTask created.
const podOwnerLabelKey = codeenginetask.GroupName + "/podOwner"

// Reconciler implements simpledeploymentreconciler.Interface for
// CodeEngineTask resources.
type Reconciler struct {
	kubeclient kubernetes.Interface
	runLister  listersalpha.RunLister
}

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, run *v1alpha1.Run) reconciler.Event {
	// This logger has all the context necessary to identify which resource is being reconciled.
	logger := logging.FromContext(ctx)
	logger.Infof("Reconciling CodeEngine task run %s/%s at %v", run.Namespace, run.Name, time.Now())

	// Ensure expected custom task reference
	if run.Spec.Ref == nil ||
		run.Spec.Ref.APIVersion != codeenginetaskv1alpha1.SchemeGroupVersion.String() ||
		run.Spec.Ref.Kind != codeenginetask.CustomTaskKind {
		logger.Errorf("Unexpected custom task Run %s/%s. Expected: %s/%s, got: %s/%s",
			run.Namespace, run.Name,
			codeenginetaskv1alpha1.SchemeGroupVersion.String(), codeenginetask.CustomTaskKind,
			run.Spec.Ref.APIVersion, run.Spec.Ref.Kind)
		return nil
	}

	//TODO: implement reconcile

	return nil
}
