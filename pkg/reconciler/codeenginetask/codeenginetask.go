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
	"time"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	"github.com/hashicorp/go-multierror"
	apisv1beta1 "github.com/rafal-bigaj/code-engine-batch-job-client/pkg/apis/codeengine/v1beta1"
	typedv1beta1 "github.com/rafal-bigaj/code-engine-batch-job-client/pkg/client/clientset/versioned/typed/codeengine/v1beta1"
	listersv1beta1 "github.com/rafal-bigaj/code-engine-batch-job-client/pkg/client/listers/codeengine/v1beta1"
	"github.com/rafalbigaj/tekton-code-engine/pkg/apis/codeenginetask"
	taskv1alpha1 "github.com/rafalbigaj/tekton-code-engine/pkg/apis/codeenginetask/v1alpha1"
	tasklisterv1alpha1 "github.com/rafalbigaj/tekton-code-engine/pkg/client/listers/codeenginetask/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	listersalpha "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
)

// Reconciler implements simpledeploymentreconciler.Interface for
// CodeEngineTask resources.
type Reconciler struct {
	kubeclient          kubernetes.Interface
	runLister           listersalpha.RunLister
	secretLister        corelisters.SecretLister
	taskLister          tasklisterv1alpha1.CodeEngineTaskLister
	jobDefinitionLister listersv1beta1.JobDefinitionLister
	jobRunLister        listersv1beta1.JobRunLister
	jobRunsClient       typedv1beta1.JobRunInterface
	codeEngineNamespace string
}

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, run *v1alpha1.Run) reconciler.Event {
	var mErr error

	// This logger has all the context necessary to identify which resource is being reconciled.
	logger := logging.FromContext(ctx)
	logger.Infof("Reconciling CodeEngine task run %s/%s at %v", run.Namespace, run.Name, time.Now())

	if run.Spec.Ref != nil && run.Spec.Spec != nil {
		logger.Errorf("Run %s/%s can provide one of Run.Spec.Ref/Run.Spec.Spec", run.Namespace, run.Name)
	}
	if run.Spec.Spec == nil && run.Spec.Ref == nil {
		logger.Errorf("Run %s/%s does not provide a spec or ref.", run.Namespace, run.Name)
		return nil
	}

	// Ensure expected custom task reference
	if run.Spec.Ref != nil &&
		(run.Spec.Ref.APIVersion != taskv1alpha1.SchemeGroupVersion.String() ||
			run.Spec.Ref.Kind != codeenginetask.CustomTaskKind) {
		logger.Errorf("Unexpected custom task Run %s/%s. Expected: %s/%s, got: %s/%s",
			run.Namespace, run.Name,
			taskv1alpha1.SchemeGroupVersion.String(), codeenginetask.CustomTaskKind,
			run.Spec.Ref.APIVersion, run.Spec.Ref.Kind)
		return nil
	}

	// Ensure expected custom task reference
	if run.Spec.Spec != nil &&
		(run.Spec.Spec.APIVersion != taskv1alpha1.SchemeGroupVersion.String() ||
			run.Spec.Spec.Kind != codeenginetask.CustomTaskKind) {
		logger.Errorf("Unexpected custom task Run %s/%s. Expected: %s/%s, got: %s/%s",
			run.Namespace, run.Name,
			taskv1alpha1.SchemeGroupVersion.String(), codeenginetask.CustomTaskKind,
			run.Spec.Spec.APIVersion, run.Spec.Spec.Kind)
		return nil
	}

	if !run.HasStarted() {
		r.initializeRun(ctx, run, logger)
	}

	if run.IsDone() {
		r.finalizeRun(ctx, run, logger)
		return nil
	}

	status := &taskv1alpha1.CodeEngineTaskStatus{}
	if err := run.Status.DecodeExtraFields(status); err != nil {
		run.Status.MarkRunFailed(taskv1alpha1.CodeEngineTaskRunReasonFailed.String(),
			"Internal error calling DecodeExtraFields: %v", err)
		logger.Errorf("DecodeExtraFields error: %v", err.Error())
	}

	// Reconcile the Run
	if err := r.reconcile(ctx, run, logger, status); err != nil {
		logger.Errorf("Reconcile error: %v", err.Error())
		mErr = multierror.Append(mErr, err)
	}

	if err := run.Status.EncodeExtraFields(status); err != nil {
		run.Status.MarkRunFailed(taskv1alpha1.CodeEngineTaskRunReasonFailed.String(),
			"Internal error calling EncodeExtraFields: %v", err)
		logger.Errorf("EncodeExtraFields error: %v", err.Error())
	}

	// Store the condition before reconcile
	beforeCondition := run.Status.GetCondition(apis.ConditionSucceeded)

	afterCondition := run.Status.GetCondition(apis.ConditionSucceeded)
	events.Emit(ctx, beforeCondition, afterCondition, run)

	return mErr
}

// initialize the Condition and set the start time.
func (r *Reconciler) initializeRun(ctx context.Context, run *v1alpha1.Run, logger *zap.SugaredLogger) {
	logger.Infof("Starting new CodeEngine task Run %s/%s", run.Namespace, run.Name)
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
	logger.Infof("Run %s/%s is done", run.Namespace, run.Name)
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

	err = r.checkCodeEngineJobRunStatus(ctx, run, status, logger)
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
			return nil, fmt.Errorf("error unmarshal Code Engine task spec for Run %s: %w", fmt.Sprintf("%s/%s", run.Namespace, run.Name), err)
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

func (r *Reconciler) runCodeEngineJob(ctx context.Context, run *v1alpha1.Run, status *taskv1alpha1.CodeEngineTaskStatus, logger *zap.SugaredLogger) error {
	// Check if the run has not been started yet.
	if status.JobRunName == "" {
		jd, err := r.jobDefinitionLister.JobDefinitions(r.codeEngineNamespace).Get(status.CodeEngineTaskSpec.JobDefinitionName)

		if err != nil {
			run.Status.MarkRunFailed(taskv1alpha1.CodeEngineTaskRunReasonFailedToStartJobRun.String(),
				"Error retrieving CodeEngine JobDefinition %q for Run %s/%s: %s",
				status.CodeEngineTaskSpec.JobDefinitionName, run.Namespace, run.Name, err)
			return err
		}

		jobEnv := []corev1.EnvVar{{
			Name:  "JOB_COUNT",
			Value: "20",
		}}

		jobRun := &apisv1beta1.JobRun{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: status.CodeEngineTaskSpec.JobDefinitionName + "-",
				Labels: map[string]string{
					jobRunOwnerLabelKey:          run.Name,
					jobRunOwnerNamespaceLabelKey: run.Namespace,
				},
			},
			Spec: apisv1beta1.JobRunSpec{
				JobDefinitionRef: status.CodeEngineTaskSpec.JobDefinitionName,
				JobDefinitionSpec: apisv1beta1.JobDefinitionSpec{
					// ArraySpec: StringPtr("1"),
					// RetryLimit: Int64Ptr(3),
					// MaxExecutionTime: Int64Ptr(7200),
					Template: apisv1beta1.JobPodTemplate{
						Containers: []corev1.Container{{
							Env: jobEnv,
							/* Image: "busybox",
							Command: []string{"/bin/sh"},
							Args: []string{"-c", "echo OK!"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							}, */
						}},
					},
				},
			},
		}

		apisv1beta1.SetDefaultsFromJobDefinition(jobRun, *jd)

		createdJobRun, err := r.jobRunsClient.Create(ctx, jobRun, metav1.CreateOptions{})
		if err != nil {
			run.Status.MarkRunFailed(taskv1alpha1.CodeEngineTaskRunReasonFailedToStartJobRun.String(),
				"Error creating CodeEngine JobRun for Run %s/%s: %s",
				run.Namespace, run.Name, err)
			return err
		}
		status.JobRunName = createdJobRun.Name

		run.Status.MarkRunRunning(taskv1alpha1.CodeEngineTaskRunReasonRunning.String(),
			"CodeEngine JobRun %q created for Run %s/%s",
			status.JobRunName, run.Namespace, run.Name)
	}
	return nil
}

func Int64Ptr(i int64) *int64 { return &i }

func StringPtr(s string) *string { return &s }

func (r *Reconciler) checkCodeEngineJobRunStatus(ctx context.Context, run *v1alpha1.Run, status *taskv1alpha1.CodeEngineTaskStatus, logger *zap.SugaredLogger) error {
	// Check if the job run has been completed
	if len(status.JobRunName) > 0 {
		jobRun, err := r.jobRunLister.JobRuns(r.codeEngineNamespace).Get(status.JobRunName)

		if err != nil {
			if errors.IsNotFound(err) {
				// ignore NotFound and wait for resource availability in informer cache
				return nil
			}
			run.Status.MarkRunFailed(taskv1alpha1.CodeEngineTaskRunReasonFailedToGetJobRunStatus.String(),
				"Error retrieving CodeEngine JobRun %q status for Run %s/%s: %s",
				status.JobRunName, run.Namespace, run.Name, err)
			return err
		}

		if jobRun.CheckCondition(apisv1beta1.JobComplete) {
			run.Status.MarkRunSucceeded(taskv1alpha1.CodeEngineTaskRunReasonSucceeded.String(),
				"CodeEngine JobRun %q completed for Run %s/%s",
				status.JobRunName, run.Namespace, run.Name)
		}

		if jobRun.CheckCondition(apisv1beta1.JobFailed) {
			run.Status.MarkRunFailed(taskv1alpha1.CodeEngineTaskRunReasonFailed.String(),
				"CodeEngine JobRun %q failed for Run %s/%s",
				status.JobRunName, run.Namespace, run.Name)
		}
	}
	return nil
}
