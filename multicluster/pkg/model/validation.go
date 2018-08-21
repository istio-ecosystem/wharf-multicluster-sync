package model

import (
	"github.com/golang/protobuf/proto"
	"fmt"

	multierror "github.com/hashicorp/go-multierror"

	multicluster "github.ibm.com/istio-research/multicluster-roadmap/api/multicluster/v1alpha1"
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
		// for _, exposed := range value.Exposed {
		// 	errs = appendErrors(errs, validateServer(server))
		// }
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
