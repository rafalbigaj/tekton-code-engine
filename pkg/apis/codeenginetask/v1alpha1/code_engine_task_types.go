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

package v1alpha1

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// CodeEngineTask is a Knative abstraction that encapsulates the interface by which Knative
// components express a desire to have a particular image cached.
//
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CodeEngineTask represents a remote batch job executed in IBM Code Engine.
// +k8s:openapi-gen=true
type CodeEngineTask struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the CodeEngineTask (from the client).
	// +optional
	Spec CodeEngineTaskSpec `json:"spec,omitempty"`

	// Status communicates the observed state of the CodeEngineTask (from the controller).
	// +optional
	Status CodeEngineTaskStatus `json:"status,omitempty"`
}

var (
	// Check that AddressableService can be validated and defaulted.
	_ apis.Validatable   = (*CodeEngineTask)(nil)
	_ apis.Defaultable   = (*CodeEngineTask)(nil)
	_ kmeta.OwnerRefable = (*CodeEngineTask)(nil)
	// Check that the type conforms to the duck Knative Resource shape.
	_ duckv1.KRShaped = (*CodeEngineTask)(nil)
)

// CodeEngineTaskSpec holds the desired state of the CodeEngineTask (from the client).
type CodeEngineTaskSpec struct {
	JobName string `json:"jobName"`
}

// CodeEngineTaskRunReason represents a reason for the Run "Succeeded" condition
type CodeEngineTaskRunReason string

const (
	// CodeEngineTaskRunReason is the reason set when the Run has just started
	CodeEngineTaskRunReasonStarted CodeEngineTaskRunReason = "Started"

	// CodeEngineTaskRunReasonRunning indicates that the Run is in progress
	CodeEngineTaskRunReasonRunning CodeEngineTaskRunReason = "Running"

	// CodeEngineTaskRunReasonFailed indicates that one of the TaskRuns created from the Run failed
	CodeEngineTaskRunReasonFailed CodeEngineTaskRunReason = "Failed"

	// CodeEngineTaskRunReasonSucceeded indicates that all of the TaskRuns created from the Run completed successfully
	CodeEngineTaskRunReasonSucceeded CodeEngineTaskRunReason = "Succeeded"

	// CodeEngineTaskRunReasonCouldntGetTask indicates that the associated CodeEngineTask couldn't be retrieved
	CodeEngineTaskRunReasonCouldntGetTask CodeEngineTaskRunReason = "CouldntGetCodeEngineTask"

	// CodeEngineTaskRunReasonFailedToStartJobRun indicates that Code Engine job run can not be created
	CodeEngineTaskRunReasonFailedToStartJobRun CodeEngineTaskRunReason = "FailedToStartJobRun"

	// CodeEngineTaskRunReasonFailedToGetJobRunStatus indicates that Code Engine job run status can not be gathered
	CodeEngineTaskRunReasonFailedToGetJobRunStatus CodeEngineTaskRunReason = "FailedToGetJobRunStatus"
)

func (t CodeEngineTaskRunReason) String() string {
	return string(t)
}

const (
	// CodeEngineTaskConditionReady is set when the revision is starting to materialize
	// runtime resources, and becomes true when those resources are ready.
	CodeEngineTaskConditionReady = apis.ConditionReady
)

// CodeEngineTaskStatus communicates the observed state of the CodeEngineTask (from the controller).
type CodeEngineTaskStatus struct {
	duckv1.Status `json:",inline"`

	// CodeEngineTaskSpec contains the exact spec used to instantiate the Run
	CodeEngineTaskSpec *CodeEngineTaskSpec `json:"codeEngineTaskSpec,omitempty"`

	// JobRunName is a name of the job run from CodeEngine
	JobRunName string `json:"jobRunName,omitempty"`
}

// CodeEngineTaskList is a list of AddressableService resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CodeEngineTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CodeEngineTask `json:"items"`
}

// GetStatus retrieves the status of the resource. Implements the KRShaped interface.
func (t *CodeEngineTask) GetStatus() *duckv1.Status {
	return &t.Status.Status
}

func DecodeStatusFromRun(run *v1alpha1.Run, logger *zap.SugaredLogger) (*CodeEngineTaskStatus, error) {
	status := &CodeEngineTaskStatus{}
	if err := run.Status.DecodeExtraFields(status); err != nil {
		run.Status.MarkRunFailed(CodeEngineTaskRunReasonFailed.String(),
			"Internal error calling DecodeExtraFields: %v", err)
		logger.Errorf("DecodeExtraFields error: %v", err)
		return nil, err
	}
	return status, nil
}

func (ts *CodeEngineTaskStatus) EncodeIntoRun(run *v1alpha1.Run, logger *zap.SugaredLogger) error {
	if err := run.Status.EncodeExtraFields(ts); err != nil {
		run.Status.MarkRunFailed(CodeEngineTaskRunReasonFailed.String(),
			"Internal error calling EncodeExtraFields: %v", err)
		logger.Errorf("EncodeExtraFields error: %v", err)
		return err
	}
	return nil
}
