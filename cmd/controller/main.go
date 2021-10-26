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

package main

import (
	"flag"
	"fmt"
	"go.uber.org/zap"
	"os"

	// The set of controllers this controller process runs.
	cecontext "github.com/rafalbigaj/tekton-code-engine/pkg/codeenegine"
	"github.com/rafalbigaj/tekton-code-engine/pkg/reconciler/codeenginetask"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	// This defines the shared main for injected controllers.
	"knative.dev/pkg/injection/sharedmain"
)

const (
	LogLevelEnv = "LOG_LEVEL"
)

func main() {
	flag.Parse()
	logger := initLogger()

	ctx := signals.NewContext()
	ctx = logging.WithLogger(ctx, logger)
	ctx = cecontext.WithClientAndInformers(ctx)

	sharedmain.MainWithContext(ctx, "controller",
		codeenginetask.NewController,
	)
}

func initLogger() *zap.SugaredLogger {
	logLevel := os.Getenv(LogLevelEnv)
	config := zap.NewProductionConfig()
	switch logLevel {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	}
	logger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("Cannot initialize logger: %v", err))
	}
	return logger.Sugar()
}
