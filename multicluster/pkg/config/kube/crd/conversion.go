//
// Modified version of istio.io/istio/pilot/pkg/config/kube/crd/conversion.go
//
// If the Multi-Cluster will be added into Istio's known types then this
// controller is not necessary as the one above can be used.

package crd

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	multiclustermodel "github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/model"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"

	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/log"
)

// ConvertObject converts an IstioObject k8s-style object to the
// internal configuration model.
func ConvertObject(schema model.ProtoSchema, object IstioObject, domain string) (*model.Config, error) {
	data, err := schema.FromJSONMap(object.GetSpec())
	if err != nil {
		return nil, err
	}
	meta := object.GetObjectMeta()

	//// FIXME this is a gross hack to hardcode a service's domain name in kubernetes
	//if domain == "" {
	//	domain = "svc.cluster.local"
	//} else {
	//	domain = "svc." + domain
	//}

	return &model.Config{
		ConfigMeta: model.ConfigMeta{
			Type:              schema.Type,
			Group:             ResourceGroup(&schema),
			Version:           schema.Version,
			Name:              meta.Name,
			Namespace:         meta.Namespace,
			Domain:            domain,
			Labels:            meta.Labels,
			Annotations:       meta.Annotations,
			ResourceVersion:   meta.ResourceVersion,
			CreationTimestamp: meta.CreationTimestamp,
		},
		Spec: data,
	}, nil
}

// ConvertConfig translates Istio config to k8s config JSON
func ConvertConfig(schema model.ProtoSchema, config model.Config) (IstioObject, error) {
	spec, err := model.ToJSONMap(config.Spec)
	if err != nil {
		return nil, err
	}
	namespace := config.Namespace
	if namespace == "" {
		namespace = meta_v1.NamespaceDefault
	}
	out := knownTypes[schema.Type].object.DeepCopyObject().(IstioObject)
	out.SetObjectMeta(meta_v1.ObjectMeta{
		Name:            config.Name,
		Namespace:       namespace,
		ResourceVersion: config.ResourceVersion,
		Labels:          config.Labels,
		Annotations:     config.Annotations,
	})
	out.SetSpec(spec)

	return out, nil
}

// ResourceName converts "my-name" to "myname".
// This is needed by k8s API server as dashes prevent kubectl from accessing CRDs
func ResourceName(s string) string {
	return strings.Replace(s, "-", "", -1)
}

// ResourceGroup generates the k8s API group for each schema.
func ResourceGroup(schema *model.ProtoSchema) string {
	return schema.Group + model.IstioAPIGroupDomain
}

// TODO - add special cases for type-to-kind and kind-to-type
// conversions with initial-isms. Consider adding additional type
// information to the abstract model and/or elevating k8s
// representation to first-class type to avoid extra conversions.

// KabobCaseToCamelCase converts "my-name" to "MyName"
func KabobCaseToCamelCase(s string) string {
	switch s {
	case "http-api-spec":
		return "HTTPAPISpec"
	case "http-api-spec-binding":
		return "HTTPAPISpecBinding"
	default:
		words := strings.Split(s, "-")
		out := ""
		for _, word := range words {
			out = out + strings.Title(word)
		}
		return out
	}
}

// CamelCaseToKabobCase converts "MyName" to "my-name"
func CamelCaseToKabobCase(s string) string {
	switch s {
	case "HTTPAPISpec":
		return "http-api-spec"
	case "HTTPAPISpecBinding":
		return "http-api-spec-binding"
	default:
		var out bytes.Buffer
		for i := range s {
			if 'A' <= s[i] && s[i] <= 'Z' {
				if i > 0 {
					out.WriteByte('-')
				}
				out.WriteByte(s[i] - 'A' + 'a')
			} else {
				out.WriteByte(s[i])
			}
		}
		return out.String()
	}
}

func parseInputsImpl(inputs string, withValidate bool) ([]model.Config, []IstioKind, error) {
	var varr []model.Config
	var others []IstioKind
	reader := bytes.NewReader([]byte(inputs))
	var empty = IstioKind{}

	// We store configs as a YaML stream; there may be more than one decoder.
	yamlDecoder := kubeyaml.NewYAMLOrJSONDecoder(reader, 512*1024)
	for {
		obj := IstioKind{}
		err := yamlDecoder.Decode(&obj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("cannot parse proto message: %v", err)
		}
		if reflect.DeepEqual(obj, empty) {
			continue
		}

		schema, exists := multiclustermodel.MultiClusterConfigTypes.GetByType(CamelCaseToKabobCase(obj.Kind))
		if !exists {
			log.Debugf("unrecognized type %v", obj.Kind)
			others = append(others, obj)
			continue
		}

		config, err := ConvertObject(schema, &obj, "")
		if err != nil {
			return nil, nil, fmt.Errorf("cannot parse proto message: %v", err)
		}

		if withValidate {
			if err := schema.Validate(config.Name, config.Namespace, config.Spec); err != nil {
				return nil, nil, fmt.Errorf("configuration is invalid: %v", err)
			}
		}

		varr = append(varr, *config)
	}

	return varr, others, nil
}

// ParseInputs reads multiple documents from `kubectl` output and checks with
// the schema. It also returns the list of unrecognized kinds as the second
// response.
//
// NOTE: This function only decodes a subset of the complete k8s
// ObjectMeta as identified by the fields in model.ConfigMeta. This
// would typically only be a problem if a user dumps an configuration
// object with kubectl and then re-ingests it.
func ParseInputs(inputs string) ([]model.Config, []IstioKind, error) {
	return parseInputsImpl(inputs, true)
}

// ParseInputsWithoutValidation same as ParseInputs, but do not apply schema validation.
func ParseInputsWithoutValidation(inputs string) ([]model.Config, []IstioKind, error) {
	return parseInputsImpl(inputs, false)
}
