module github.com/rafalbigaj/tekton-code-engine

go 1.14

require (
	github.com/IBM/code-engine-go-sdk v0.0.0-00010101000000-000000000000
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/google/licenseclassifier v0.0.0-20210108172934-df6aa8a2788b
	github.com/hashicorp/go-multierror v1.1.0
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/tektoncd/pipeline v0.20.0
	go.uber.org/zap v1.16.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/code-generator v0.20.2
	k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd
	knative.dev/hack v0.0.0-20210114150620-4422dcadb3c8
	knative.dev/pkg v0.0.0-20210118192521-75d66b58948d
)

replace (
	github.com/IBM/code-engine-go-sdk => ../code-engine-go-sdk

	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2

	k8s.io/api => k8s.io/api v0.20.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.2
	k8s.io/client-go => k8s.io/client-go v0.20.2
	k8s.io/code-generator => k8s.io/code-generator v0.20.2
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd
)
