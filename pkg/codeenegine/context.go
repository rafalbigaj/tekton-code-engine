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

	"github.com/IBM/code-engine-go-sdk/pkg/client/clientset/versioned"
	"github.com/IBM/code-engine-go-sdk/pkg/client/clientset/versioned/typed/codeengine/v1beta1"
	"github.com/IBM/code-engine-go-sdk/pkg/client/informers/externalversions"
	informersv1beta1 "github.com/IBM/code-engine-go-sdk/pkg/client/informers/externalversions/codeengine/v1beta1"
	"go.uber.org/zap"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// optionsKey is used as the key for associating information
// with a context.Context.
type optionsKey struct{}

// clientKey is used as the key for associating information
// with a context.Context.
type clientKey struct{}

// jobDefinitionInformerKey is used as the key for associating information
// with a context.Context.
type jobDefinitionInformerKey struct{}

// jobRunInformerKey is used as the key for associating information
// with a context.Context.
type jobRunInformerKey struct{}

// Options contains the configuration for the webhook
type Options struct {
	// Kubeconfig is the kubeconfig filepath.
	Kubeconfig string
}

// WithOptions associates a set of webhook.Options with
// the returned context.
func WithOptions(ctx context.Context, opt Options) context.Context {
	ctx = context.WithValue(ctx, optionsKey{}, &opt)
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

// GetOptions retrieves webhook.Options associated with the
// given context via WithOptions (above).
func GetOptions(ctx context.Context) *Options {
	v := ctx.Value(optionsKey{})
	if v == nil {
		return nil
	}
	return v.(*Options)
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
	return GetClient(ctx).CodeengineV1beta1().JobRuns("5wijbqm1mq4")
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

func startCodeEngineInformers(ctx context.Context) context.Context {
	logger := logging.FromContext(ctx)

	opts := GetOptions(ctx)

	codeEngineCfg, err := injection.GetRESTConfig("", opts.Kubeconfig)
	if err != nil {
		logger.Fatalw("", zap.Error(err))
	}

	client := versioned.NewForConfigOrDie(codeEngineCfg)
	ctx = withClient(ctx, client)

	factory := externalversions.NewSharedInformerFactoryWithOptions(client, controller.GetResyncPeriod(ctx),
		externalversions.WithNamespace("5wijbqm1mq4"))

	jobDefinitionInformer := factory.Codeengine().V1beta1().JobDefinitions()
	jobRunInformer := factory.Codeengine().V1beta1().JobRuns()

	logging.FromContext(ctx).Info("Starting CodeEngine informers...")
	if err := controller.StartInformers(ctx.Done(), jobDefinitionInformer.Informer(), jobRunInformer.Informer()); err != nil {
		logging.FromContext(ctx).Fatalw("Failed to start CodeEngine informers", zap.Error(err))
	}

	return withInformers(ctx, jobDefinitionInformer, jobRunInformer)
}
