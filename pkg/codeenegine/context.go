/*
Copyright 2021 The Knative Authors

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

package context

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/IBM/code-engine-go-sdk/ibmcloudcodeenginev1"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/rafalbigaj/code-engine-batch-job-client/pkg/client/clientset/versioned"
	"github.com/rafalbigaj/code-engine-batch-job-client/pkg/client/clientset/versioned/typed/codeengine/v1beta1"
	"github.com/rafalbigaj/code-engine-batch-job-client/pkg/client/informers/externalversions"
	informersv1beta1 "github.com/rafalbigaj/code-engine-batch-job-client/pkg/client/informers/externalversions/codeengine/v1beta1"
	"go.uber.org/zap"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const (
	CEApiKeyEnv        = "CE_API_KEY"
	CEProjectRegionEnv = "CE_PROJECT_REGION"
	CEProjectIdEnv     = "CE_PROJECT_ID"
)

// namespaceKey is used as the key for associating information
// with a context.Context.
type namespaceKey struct{}

// clientKey is used as the key for associating information
// with a context.Context.
type clientKey struct{}

// jobDefinitionInformerKey is used as the key for associating information
// with a context.Context.
type jobDefinitionInformerKey struct{}

// jobRunInformerKey is used as the key for associating information
// with a context.Context.
type jobRunInformerKey struct{}

// WithClient initializes Code Engine client and informers.
func WithClientAndInformers(ctx context.Context) context.Context {
	return startCodeEngineInformers(ctx)
}

// withInformers associates a set of CodeEngine informers with
// the returned context.
func withInformers(ctx context.Context,
	jobDefinitionInformer informersv1beta1.JobDefinitionInformer,
	jobRunInformer informersv1beta1.JobRunInformer) context.Context {
	ctx = context.WithValue(ctx, jobDefinitionInformerKey{}, jobDefinitionInformer)
	ctx = context.WithValue(ctx, jobRunInformerKey{}, jobRunInformer)
	return ctx
}

// withClient associates a set of CodeEngine clientset with
// the returned context.
func withClient(ctx context.Context, client *versioned.Clientset) context.Context {
	return context.WithValue(ctx, clientKey{}, client)
}

// withNamespace associates a set of CodeEngine clientset with
// the returned context.
func withNamespace(ctx context.Context, ns string) context.Context {
	return context.WithValue(ctx, namespaceKey{}, ns)
}

// GetNamespace retrieves webhook.Options associated with the
// given context via WithOptions (above).
func GetNamespace(ctx context.Context) string {
	v := ctx.Value(namespaceKey{})
	if v == nil {
		return ""
	}
	return v.(string)
}

// GetClient retrieves CodeEngine clientset associated with the
// given context via withClient (above).
func GetClient(ctx context.Context) *versioned.Clientset {
	v := ctx.Value(clientKey{})
	if v == nil {
		return nil
	}
	return v.(*versioned.Clientset)
}

// GetJobRunsClient retrieves CodeEngine JobRuns associated with the
// given context via withClient (above).
func GetJobRunsClient(ctx context.Context) v1beta1.JobRunInterface {
	ns := GetNamespace(ctx)
	return GetClient(ctx).CodeengineV1beta1().JobRuns(ns)
}

// GetJobDefinitionInformer retrieves JobDefinitionInformer associated with the
// given context via withInformers (above).
func GetJobDefinitionInformer(ctx context.Context) informersv1beta1.JobDefinitionInformer {
	v := ctx.Value(jobDefinitionInformerKey{})
	if v == nil {
		return nil
	}
	return v.(informersv1beta1.JobDefinitionInformer)
}

// GetJobRunInformer retrieves JobRunInformer associated with the
// given context via withInformers (above).
func GetJobRunInformer(ctx context.Context) informersv1beta1.JobRunInformer {
	v := ctx.Value(jobRunInformerKey{})
	if v == nil {
		return nil
	}
	return v.(informersv1beta1.JobRunInformer)
}

func getCodeEngineKubeConfig(ctx context.Context) (clientcmd.ClientConfig, error) {
	logger := logging.FromContext(ctx)

	// Validate environment
	requiredEnvs := []string{CEApiKeyEnv, CEProjectRegionEnv, CEProjectIdEnv}
	for _, envName := range requiredEnvs {
		val := os.Getenv(envName)
		if val == "" {
			return nil, fmt.Errorf("environment variable %s must be set", envName)
		}
		logger.Debugf("ENV[%s]: %s", envName, val)
	}

	// Create an IAM authenticator.
	authenticator := &core.IamAuthenticator{
		ApiKey:       os.Getenv(CEApiKeyEnv),
		ClientId:     "bx",
		ClientSecret: "bx",
	}

	// Setup a Code Engine client
	ceClient, err := ibmcloudcodeenginev1.NewIbmCloudCodeEngineV1(&ibmcloudcodeenginev1.IbmCloudCodeEngineV1Options{
		Authenticator: authenticator,
		URL:           "https://api." + os.Getenv(CEProjectRegionEnv) + ".codeengine.cloud.ibm.com/api/v1",
	})
	if err != nil {
		return nil, fmt.Errorf("unable to setup Code Engine client %v", err)
	}

	// Use the http library to get an IAM Delegated Refresh Token
	iamRequestData := url.Values{}
	iamRequestData.Set("grant_type", "urn:ibm:params:oauth:grant-type:apikey")
	iamRequestData.Set("apikey", os.Getenv(CEApiKeyEnv))
	iamRequestData.Set("response_type", "delegated_refresh_token")
	iamRequestData.Set("receiver_client_ids", "ce")
	iamRequestData.Set("delegated_refresh_token_expiry", "3600")

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://iam.cloud.ibm.com/identity/token", strings.NewReader(iamRequestData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("unable to create IAM token request for Code Engine client %v", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to get delegated IAM token for Code Engine client %v", err)
	}

	var iamResponseData map[string]string
	json.NewDecoder(resp.Body).Decode(&iamResponseData)
	resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		errorCode := iamResponseData["errorCode"]
		errorMessage := iamResponseData["errorMessage"]
		return nil, fmt.Errorf("unable to get IAM token [%s]: %s", errorCode, errorMessage)
	}

	delegatedRefreshToken := iamResponseData["delegated_refresh_token"]

	// Get Code Engine project config using the Code Engine Client
	projectID := os.Getenv(CEProjectIdEnv)
	result, _, err := ceClient.GetKubeconfigWithContext(ctx, &ibmcloudcodeenginev1.GetKubeconfigOptions{
		XDelegatedRefreshToken: &delegatedRefreshToken,
		ID:                     &projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get kubeconfig for Code Engine project: %v, %v", projectID, err)
	}

	// Get Kubernetes client using Code Engine project config
	kubeClientConfig, err := clientcmd.NewClientConfigFromBytes([]byte(*result))
	if err != nil {
		return nil, fmt.Errorf("unable to load Kubernetes config retrived from Code Engine project %v: %v", projectID, err)
	}
	return kubeClientConfig, nil
}

func startCodeEngineInformers(ctx context.Context) context.Context {
	logger := logging.FromContext(ctx)

	codeEngineClientConfig, err := getCodeEngineKubeConfig(ctx)

	if err != nil {
		logger.Fatalw("Unable to load Code Engine kubernetes config", zap.Error(err))
	}

	codeEngineCfg, err := codeEngineClientConfig.ClientConfig()
	if err != nil {
		logger.Fatalw("Unable to get ClientConfig from Code Engine kubernetes config", zap.Error(err))
	}

	client := versioned.NewForConfigOrDie(codeEngineCfg)
	ctx = withClient(ctx, client)

	namespace, _, err := codeEngineClientConfig.Namespace()
	if err != nil {
		logger.Fatalw("Unable to get namespace from Code Engine kubernetes config", zap.Error(err))
	}

	factory := externalversions.NewSharedInformerFactoryWithOptions(client, controller.GetResyncPeriod(ctx),
		externalversions.WithNamespace(namespace))

	jobDefinitionInformer := factory.Codeengine().V1beta1().JobDefinitions()
	jobRunInformer := factory.Codeengine().V1beta1().JobRuns()

	logger.Info("Starting CodeEngine informers...")
	if err := controller.StartInformers(ctx.Done(), jobDefinitionInformer.Informer(), jobRunInformer.Informer()); err != nil {
		logger.Fatalw("Failed to start CodeEngine informers", zap.Error(err))
	}

	ctx = withNamespace(ctx, namespace)

	return withInformers(ctx, jobDefinitionInformer, jobRunInformer)
}
