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
	"encoding/json"
	"fmt"
	"knative.dev/pkg/controller"

	gocache "github.com/patrickmn/go-cache"
	apisv1beta1 "github.com/rafalbigaj/code-engine-batch-job-client/pkg/apis/codeengine/v1beta1"
	typedv1beta1 "github.com/rafalbigaj/code-engine-batch-job-client/pkg/client/clientset/versioned/typed/codeengine/v1beta1"
	listersv1beta1 "github.com/rafalbigaj/code-engine-batch-job-client/pkg/client/listers/codeengine/v1beta1"
	"github.com/rafalbigaj/tekton-code-engine/pkg/apis/codeenginetask"
	taskv1alpha1 "github.com/rafalbigaj/tekton-code-engine/pkg/apis/codeenginetask/v1alpha1"
	tasklisterv1alpha1 "github.com/rafalbigaj/tekton-code-engine/pkg/client/listers/codeenginetask/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	listersalpha "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

// Reconciler implements simpledeploymentreconciler.Interface for
// CodeEngineTask resources.
type Reconciler struct {
	kubeclient kubernetes.Interface

	// Listers for Tekton resources
	runLister    listersalpha.RunLister
	secretLister corelisters.SecretLister
	taskLister   tasklisterv1alpha1.CodeEngineTaskLister

	// Listers and clients for Code Engine resources
	jobDefinitionLister listersv1beta1.JobDefinitionLister
	jobRunLister        listersv1beta1.JobRunLister
	jobRunsClient       typedv1beta1.JobRunInterface
	codeEngineNamespace string

	// Local cache for already created job runs
	createdJobRuns *gocache.Cache
}

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, run *v1alpha1.Run) reconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Info("Reconciling CodeEngine Run")

	// Make sure CodeEngine resource is being reconciled. Ignore others.
	if !r.verifyCodeEngineSpec(run, logger) {
		return nil
	}

	// Store the condition before reconcile
	beforeCondition := run.Status.GetCondition(apis.ConditionSucceeded)
	defer func() {
		afterCondition := run.Status.GetCondition(apis.ConditionSucceeded)
		events.Emit(ctx, beforeCondition, afterCondition, run)
	}()

	if !run.HasStarted() {
		r.initializeRun(ctx, run, logger)
	}

	if run.IsDone() {
		r.finalizeRun(ctx, run, logger)
	} else {
		status, err := taskv1alpha1.DecodeStatusFromRun(run, logger)
		if err != nil {
			return err
		}

		// Reconcile the Run
		err = r.reconcile(ctx, run, logger, status)
		if err != nil {
			return err
		}

		err = status.EncodeIntoRun(run, logger)
		if err != nil {
			return err
		}
	}

	return nil
}

// check if Run Spec refers to CodeEngine custom resource (embedded or standalone)
func (r *Reconciler) verifyCodeEngineSpec(run *v1alpha1.Run, logger *zap.SugaredLogger) bool {
	if run.Spec.Ref != nil && run.Spec.Spec != nil {
		logger.Error("Run has to provide one of Run.Spec.Ref/Run.Spec.Spec")
	}
	if run.Spec.Spec == nil && run.Spec.Ref == nil {
		logger.Error("Run does not provide neither task spec nor reference")
		return false
	}

	// Ensure expected custom task reference
	if run.Spec.Ref != nil &&
		(run.Spec.Ref.APIVersion != taskv1alpha1.SchemeGroupVersion.String() ||
			run.Spec.Ref.Kind != codeenginetask.CustomTaskKind) {
		logger.Errorf("Unexpected custom task kind: %s/%s, expected: %s/%s",
			taskv1alpha1.SchemeGroupVersion.String(), codeenginetask.CustomTaskKind,
			run.Spec.Ref.APIVersion, run.Spec.Ref.Kind)
		return false
	}

	// Ensure expected custom task reference
	if run.Spec.Spec != nil &&
		(run.Spec.Spec.APIVersion != taskv1alpha1.SchemeGroupVersion.String() ||
			run.Spec.Spec.Kind != codeenginetask.CustomTaskKind) {
		logger.Errorf("Unexpected custom task kind: %s/%s, expected: %s/%s",
			taskv1alpha1.SchemeGroupVersion.String(), codeenginetask.CustomTaskKind,
			run.Spec.Ref.APIVersion, run.Spec.Ref.Kind)
		return false
	}

	return true
}

// initialize the Condition and set the start time.
func (r *Reconciler) initializeRun(ctx context.Context, run *v1alpha1.Run, logger *zap.SugaredLogger) {
	logger.Info("Initializing CodeEngine Run status")
	run.Status.InitializeConditions()
	// In case node time was not synchronized, when controller has been scheduled to other nodes.
	if run.Status.StartTime.Sub(run.CreationTimestamp.Time) < 0 {
		logger.Warnf("Run %s createTimestamp %s is after the Run started %s", run.Name, run.CreationTimestamp, run.Status.StartTime)
		run.Status.StartTime = &run.CreationTimestamp
	}
	// Emit events.
	afterCondition := run.Status.GetCondition(apis.ConditionSucceeded)
	events.Emit(ctx, nil, afterCondition, run)
}

func (r *Reconciler) finalizeRun(ctx context.Context, run *v1alpha1.Run, logger *zap.SugaredLogger) {
	logger.Infof("CodeEngine Run is done")
}

func (r *Reconciler) reconcile(ctx context.Context, run *v1alpha1.Run, logger *zap.SugaredLogger, status *taskv1alpha1.CodeEngineTaskStatus) error {
	// Get the CodeEngineTask referenced by the Run
	task, err := r.getCodeEngineTask(ctx, run)
	if err != nil {
		return err
	}

	storeCodeEngineTaskSpec(status, &task.Spec)

	err = r.runCodeEngineJob(ctx, run, status, logger)
	if err != nil {
		return err
	}

	err = r.checkCodeEngineJobRunStatus(run, status, logger)
	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) getCodeEngineTask(ctx context.Context, run *v1alpha1.Run) (*taskv1alpha1.CodeEngineTask, error) {
	if run.Spec.Spec != nil {
		// use embedded task spec
		taskSpec := taskv1alpha1.CodeEngineTaskSpec{}
		err := json.Unmarshal(run.Spec.Spec.Spec.Raw, &taskSpec)
		if err != nil {
			return nil, fmt.Errorf("error unmarshal Code Engine task spec: %w", err)
		}
		return &taskv1alpha1.CodeEngineTask{
			TypeMeta: metav1.TypeMeta{
				Kind:       run.Spec.Spec.Kind,
				APIVersion: run.Spec.Spec.APIVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Labels:      run.Spec.Spec.Metadata.Labels,
				Annotations: run.Spec.Spec.Metadata.Annotations,
			},
			Spec: taskSpec,
		}, nil
	} else if run.Spec.Ref != nil && run.Spec.Ref.Name != "" {
		// TODO: Use the k8 client to get the CodeEngineTask rather than the lister. This avoids a timing issue where
		// the CodeEngineTask is not yet in the lister cache if it is created at nearly the same time as the Run.
		// See https://github.com/tektoncd/pipeline/issues/2740 for discussion on this issue.
		task, err := r.taskLister.CodeEngineTasks(run.Namespace).Get(run.Spec.Ref.Name)
		if err != nil {
			run.Status.MarkRunFailed(taskv1alpha1.CodeEngineTaskRunReasonCouldntGetTask.String(),
				"Error retrieving CodeEngineTask for Run %s/%s: %s",
				run.Namespace, run.Name, err)
			return nil, fmt.Errorf("missing spec.ref.name for Run %s", fmt.Sprintf("%s/%s", run.Namespace, run.Name))
		}
		return task, nil
	} else {
		run.Status.MarkRunFailed(taskv1alpha1.CodeEngineTaskRunReasonCouldntGetTask.String(),
			"Missing spec.ref.name for Run %s/%s",
			run.Namespace, run.Name)
		return nil, fmt.Errorf("missing spec.ref.name for Run %s", fmt.Sprintf("%s/%s", run.Namespace, run.Name))
	}
}

func storeCodeEngineTaskSpec(status *taskv1alpha1.CodeEngineTaskStatus, tls *taskv1alpha1.CodeEngineTaskSpec) {
	// Only store the CodeEngineTaskSpec once, if it has never been set before.
	if status.CodeEngineTaskSpec == nil {
		status.CodeEngineTaskSpec = tls
	}
}

func (r *Reconciler) runCodeEngineJob(
	ctx context.Context,
	run *v1alpha1.Run,
	status *taskv1alpha1.CodeEngineTaskStatus,
	logger *zap.SugaredLogger,
) error {
	// Check if the run has not been started yet.
	if status.JobRunName == "" {
		cacheKey := fmt.Sprintf("%s/%s", run.Namespace, run.Name)
		if _, alreadyCreated := r.createdJobRuns.Get(cacheKey); alreadyCreated {
			// the local client cache is obsolete, let's wait for the next reconciliation
			return controller.NewRequeueImmediately()
		}

		logger.Infof("Starting a new run for CodeEngine job: %q", status.CodeEngineTaskSpec.JobName)
		jd, err := r.jobDefinitionLister.JobDefinitions(r.codeEngineNamespace).Get(status.CodeEngineTaskSpec.JobName)

		if err != nil {
			run.Status.MarkRunFailed(taskv1alpha1.CodeEngineTaskRunReasonFailedToStartJobRun.String(),
				"Error retrieving CodeEngine JobDefinition %q for Run %s/%s: %v",
				status.CodeEngineTaskSpec.JobName, run.Namespace, run.Name, err)
			return err
		}

		jobRun := &apisv1beta1.JobRun{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: status.CodeEngineTaskSpec.JobName + "-",
				Labels: map[string]string{
					jobRunOwnerLabelKey:          run.Name,
					jobRunOwnerNamespaceLabelKey: run.Namespace,
				},
			},
			Spec: apisv1beta1.JobRunSpec{
				JobDefinitionRef: status.CodeEngineTaskSpec.JobName,
			},
		}
		apisv1beta1.SetDefaultsFromJobDefinition(jobRun, *jd)

		createdJobRun, err := r.jobRunsClient.Create(ctx, jobRun, metav1.CreateOptions{})
		if err != nil {
			run.Status.MarkRunFailed(taskv1alpha1.CodeEngineTaskRunReasonFailedToStartJobRun.String(),
				"Error creating CodeEngine JobRun: %v", err)
			return err
		}
		status.JobRunName = createdJobRun.Name

		run.Status.MarkRunRunning(taskv1alpha1.CodeEngineTaskRunReasonRunning.String(),
			"Created CodeEngine JobRun: %q", status.JobRunName)
	}
	return nil
}

func (r *Reconciler) checkCodeEngineJobRunStatus(
	run *v1alpha1.Run,
	status *taskv1alpha1.CodeEngineTaskStatus,
	logger *zap.SugaredLogger,
) error {
	// Check if the job run has been completed
	if len(status.JobRunName) > 0 {
		logger.Infof("Checking the state of CodeEngine JobRun: %q", status.JobRunName)
		jobRun, err := r.jobRunLister.JobRuns(r.codeEngineNamespace).Get(status.JobRunName)

		if err != nil {
			if errors.IsNotFound(err) {
				// ignore NotFound and wait for resource availability in informer cache
				return nil
			}
			logger.Errorf("Error retriving CodeEngine JobRun %q status: %v", status.JobRunName, err)
			// mark run as still running, but inaccessible
			run.Status.MarkRunRunning(taskv1alpha1.CodeEngineTaskRunReasonFailedToGetJobRunStatus.String(),
				"Error retrieving CodeEngine JobRun %q status: %v", status.JobRunName, err)
			return err
		}

		logger.Infof("CodeEngine JobRun %q state: %s", status.JobRunName, JobRunStatus(&jobRun.Status))

		if jobRun.CheckCondition(apisv1beta1.JobComplete) {
			run.Status.MarkRunSucceeded(taskv1alpha1.CodeEngineTaskRunReasonSucceeded.String(),
				"CodeEngine JobRun %q has completed", status.JobRunName)
		}

		if jobRun.CheckCondition(apisv1beta1.JobFailed) {
			run.Status.MarkRunFailed(taskv1alpha1.CodeEngineTaskRunReasonFailed.String(),
				"CodeEngine JobRun %q has failed", status.JobRunName)
		}
	}
	return nil
}

func JobRunStatus(js *apisv1beta1.JobRunStatus) string {
	status := "Unknown"
	if cond := js.GetLatestCondition(); cond != nil {
		status = string(cond.Type)
	}
	return fmt.Sprintf("%s (unknown: %d, requested: %d, pending: %d, running: %d, succeeded: %d, failed: %d)",
		status, js.Unknown, js.Requested, js.Pending, js.Running, js.Succeeded, js.Failed)
}
