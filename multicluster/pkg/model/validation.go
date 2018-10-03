package model

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	multierror "github.com/hashicorp/go-multierror"

	istiomodel "istio.io/istio/pilot/pkg/model"

	multicluster "github.com/istio-ecosystem/wharf-multicluster-sync/api/multicluster/v1alpha1"
)

// ValidateServiceExpositionPolicy checks service exposition policy specifications
func ValidateServiceExpositionPolicy(name, namespace string, msg proto.Message) (errs error) {
	value, ok := msg.(*multicluster.ServiceExpositionPolicy)
	if !ok {
		errs = appendErrors(errs, fmt.Errorf("cannot cast to ServiceExpositionPolicy: %#v", msg))
		return
	}

	if len(value.Exposed) == 0 {
		errs = appendErrors(errs, fmt.Errorf("policy must have at least one exposition"))
	} else {
		for _, exposed := range value.Exposed {
			errs = appendErrors(errs, validateExposedService(exposed))
		}
	}

	return errs
}

func validateExposedService(vs *multicluster.ServiceExpositionPolicy_ExposedService) error {
	var errs error
	if !istiomodel.IsDNS1123Label(vs.Name) {
		errs = multierror.Append(errs, fmt.Errorf("invalid name: %q", vs.Name))
	}
	if vs.Alias != "" && !istiomodel.IsDNS1123Label(vs.Alias) {
		errs = multierror.Append(errs, fmt.Errorf("invalid alias: %q", vs.Alias))
	}

	// TODO should we validate that at least one Cluster is present?  Or does leaving
	// "Clusters" empty mean any cluster can talk to the service (public API)?
	return errs
}

// ValidateRemoteServiceBinding checks remote service binding specifications
func ValidateRemoteServiceBinding(name, namespace string, msg proto.Message) (errs error) {
	value, ok := msg.(*multicluster.RemoteServiceBinding)
	if !ok {
		errs = appendErrors(errs, fmt.Errorf("cannot cast to RemoteServiceBinding: %#v", msg))
		return
	}

	if len(value.Remote) == 0 {
		errs = appendErrors(errs, fmt.Errorf("binding must have at least one remote cluster"))
	}

	for _, remoteCluster := range value.Remote {
		if remoteCluster.GetCluster() == "" {
			errs = appendErrors(errs, fmt.Errorf("cluster cannot be empty"))
		}
	}

	return errs
}

// wrapper around multierror.Append that enforces the invariant that if all input errors are nil, the output
// error is nil (allowing validation without branching).
func appendErrors(err error, errs ...error) error {
	appendError := func(err, err2 error) error {
		if err == nil {
			return err2
		} else if err2 == nil {
			return err
		}
		return multierror.Append(err, err2)
	}

	for _, err2 := range errs {
		err = appendError(err, err2)
	}
	return err
}
