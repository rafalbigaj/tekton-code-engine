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
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"github.com/rafalbigaj/tekton-code-engine/pkg/apis/codeenginetask"
	codeenginetaskv1alpha1 "github.com/rafalbigaj/tekton-code-engine/pkg/apis/codeenginetask/v1alpha1"
	runinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/run"
	runreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1alpha1/run"
	pipelinecontroller "github.com/tektoncd/pipeline/pkg/controller"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

// NewController creates a Reconciler and returns the result of NewImpl.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	// Obtain an informer to the Run resource.
	runInformer := runinformer.Get(ctx)

	r := &Reconciler{
		// The client will be needed to create/delete Pods via the API.
		kubeclient: kubeclient.Get(ctx),
		// A lister allows read-only access to the informer's cache, allowing us to cheaply
		// read run data.
		runLister: runInformer.Lister(),
	}
	impl := runreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			AgentName: "run-code-engine-task",
		}
	})

	logger.Info("Setting up event handlers.")

	// Listen for events on the main resource and enqueue themselves.
	runInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: pipelinecontroller.FilterRunRef(codeenginetaskv1alpha1.SchemeGroupVersion.String(), codeenginetask.CustomTaskKind),
		Handler:    controller.HandleAll(impl.Enqueue),
	})

	return impl
}
