The generator tool and the template has been taken from Istio's Pilot.
The tool will generate the `types.go` used by the controller according to the config model defined in `multicluster/model/config.go`.

To re-generate the `types.go` (after modifying the `config.go`) execute the following command from the root folder:
```command
go run multicluster/tools/generate_config_crd_types.go -output multicluster/pkg/config/kube/crd/types.go
```

When/if this code will be part of Istio then the Pilot tool can be used instead of this one.