module github.com/rafalbigaj/tekton-code-engine

go 1.16

require (
	github.com/IBM/code-engine-go-sdk v0.0.0-20210223212134-4f6d4a5ffb64
	github.com/IBM/go-sdk-core/v4 v4.10.0
	github.com/google/licenseclassifier v0.0.0-20210108172934-df6aa8a2788b
	github.com/hashicorp/go-multierror v1.1.1
	github.com/rafal-bigaj/code-engine-batch-job-client v0.0.0-00010101000000-000000000000
	github.com/tektoncd/pipeline v0.28.0
	go.uber.org/zap v1.19.1
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/code-generator v0.21.4
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e
	knative.dev/hack v0.0.0-20210806075220-815cd312d65c
	knative.dev/pkg v0.0.0-20210919202233-5ae482141474
)

replace (
	github.com/rafal-bigaj/code-engine-batch-job-client => ../code-engine-batch-job-client
	k8s.io/client-go => k8s.io/client-go v0.21.4
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd
)
