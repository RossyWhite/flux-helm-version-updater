module github.com/RossyWhite/flux-helm-version-updater

go 1.16

require (
	github.com/fluxcd/helm-controller/api v0.11.2
	github.com/fluxcd/source-controller/api v0.15.4
	github.com/go-git/go-git/v5 v5.4.2
	github.com/google/go-github/v38 v38.1.0
	github.com/hashicorp/go-version v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	helm.sh/helm/v3 v3.6.3
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/kube-openapi v0.0.0-20210817084001-7fbd8d59e5b8
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/controller-runtime v0.10.0
	sigs.k8s.io/kustomize/kyaml v0.11.1
	sigs.k8s.io/yaml v1.2.0
)
