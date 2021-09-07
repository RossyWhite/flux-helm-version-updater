package setter

import (
	"errors"
	"fmt"
	"golang.org/x/xerrors"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kyaml/fieldmeta"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/setters2"
)

var (
	ErrMarkNotFound = errors.New("no marks were found")
)

func init() {
	fieldmeta.SetShortHandRef("$helmversionupdate")
}

// Execute traverses given path, and applies the set filter to nodes which is marked by `$helmversionupdate`
func Execute(path string, key client.ObjectKey, value string) error {
	settersSchema := new(spec.Schema)
	setterKey := fmt.Sprintf("%s:%s", key.Namespace, key.Name)

	settersSchema.Definitions = map[string]spec.Schema{
		fieldmeta.SetterDefinitionPrefix + setterKey: newSetterSchema(setterKey, value),
	}

	instance := &setters2.Set{Name: setterKey, SettersSchema: settersSchema}

	p := &kio.Pipeline{
		Inputs: []kio.Reader{&kio.LocalPackageReader{
			PackagePath: path,
		}},
		Outputs: []kio.Writer{&kio.LocalPackageReadWriter{
			PackagePath: path,
		}},
		Filters: []kio.Filter{
			setters2.SetAll(instance),
		},
	}

	if err := p.Execute(); err != nil {
		return xerrors.Errorf("failed to execute setter: %+w", err)
	}

	if instance.Count == 0 {
		return ErrMarkNotFound
	}

	return nil
}

// newSetterSchema returns a setter defined in OpenAPI x-k8s-cli extension.
func newSetterSchema(name, value string) spec.Schema {
	schema := spec.StringProperty()
	schema.Extensions = map[string]interface{}{}
	schema.Extensions.Add(setters2.K8sCliExtensionKey, map[string]interface{}{
		"setter": map[string]string{
			"name":  name,
			"value": value,
		},
	})
	return *schema
}
