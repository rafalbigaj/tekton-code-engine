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
	apisv1beta1 "github.com/rafalbigaj/code-engine-batch-job-client/pkg/apis/codeengine/v1beta1"
	"github.com/rafalbigaj/tekton-code-engine/pkg/apis/codeenginetask"
	codeenginetaskv1alpha1 "github.com/rafalbigaj/tekton-code-engine/pkg/apis/codeenginetask/v1alpha1"
	cetaskinformer "github.com/rafalbigaj/tekton-code-engine/pkg/client/injection/informers/codeenginetask/v1alpha1/codeenginetask"
	cecontext "github.com/rafalbigaj/tekton-code-engine/pkg/codeenegine"
	runinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/run"
	runreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1alpha1/run"
	pipelinecontroller "github.com/tektoncd/pipeline/pkg/controller"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
	"knative.dev/pkg/logging"
)

// jobRunOwnerLabelKey is the key to a label that points to the owner (creator) of the
// job run, allowing us to easily link CodeEngine job run with Tekton run.
const jobRunOwnerLabelKey = codeenginetask.GroupName + "/runOwner"

// jobRunOwnerNamespaceLabelKey is the key to a label that points to the owner namespace(creator).
const jobRunOwnerNamespaceLabelKey = codeenginetask.GroupName + "/runNamespace"

// NewController creates a Reconciler and returns the result of NewImpl.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	// Obtain an informer to the Run resource.
	runInformer := runinformer.Get(ctx)
	// Obtain an informer to the Secret resource.
	secretInformer := secretinformer.Get(ctx)
	// Obtain an informer to the CodeEngineTask resource.
	taskInformer := cetaskinformer.Get(ctx)
	// Obtain an informer to the jobDefinition resource from CodeEngine.
	jobDefinitionInformer := cecontext.GetJobDefinitionInformer(ctx)
	// Obtain an informer to the JobRun resource from CodeEngine.
	jobRunInformer := cecontext.GetJobRunInformer(ctx)
	// Obtain a client to JobRun resources from CodeEngine.
	jobRunsClient := cecontext.GetJobRunsClient(ctx)
	// Obtain a namespace from CodeEngine project.
	codeEngineNamespace := cecontext.GetNamespace(ctx)

	r := &Reconciler{
		// The client will be needed to create/delete Pods via the API.
		kubeclient: kubeclient.Get(ctx),
		// A lister allows read-only access to the informer's cache, allowing us to cheaply
		// read run data.
		runLister: runInformer.Lister(),
		// A lister allows read-only access to the informer's cache, allowing us to cheaply
		// read secret data.
		secretLister: secretInformer.Lister(),
		// A lister allows read-only access to the informer's cache, allowing us to cheaply
		// read code engine task data.
		taskLister: taskInformer.Lister(),
		// A lister allows read-only access to the informer's cache, allowing us to cheaply
		// read code engine task data.
		jobRunLister: jobRunInformer.Lister(),
		// A lister allows read-only access to the informer's cache, allowing us to cheaply
		// read code engine task data.
		jobDefinitionLister: jobDefinitionInformer.Lister(),
		// A client allows to manage code engine job runs.
		jobRunsClient: jobRunsClient,
		// A namespace representing CodeEngine project.
		codeEngineNamespace: codeEngineNamespace,
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

	// Listen for events on CodeEngine JobRun resource and enqueue themselves.
	jobRunInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: filterJobRunRef,
		Handler:    controller.HandleAll(jobRunEnqueueFunc(impl)),
	})

	return impl
}

func filterJobRunRef(obj interface{}) bool {
	jobRun, ok := obj.(*apisv1beta1.JobRun)
	if !ok {
		// Somehow got informed of a non-Run object.
		// Ignore.
		return false
	}
	_, hasOwnerLabel := jobRun.Labels[jobRunOwnerLabelKey]
	_, hasOwnerNamespaceLabel := jobRun.Labels[jobRunOwnerNamespaceLabelKey]
	return hasOwnerLabel && hasOwnerNamespaceLabel
}

func jobRunEnqueueFunc(impl *controller.Impl) func(interface{}) {
	return func(obj interface{}) {
		jobRun := obj.(*apisv1beta1.JobRun)
		runName := jobRun.Labels[jobRunOwnerLabelKey]
		runNamespace := jobRun.Labels[jobRunOwnerNamespaceLabelKey]
		impl.EnqueueKey(types.NamespacedName{Namespace: runNamespace, Name: runName})
	}
}
