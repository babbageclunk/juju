// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package apiserver

import (
	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/network"
	"github.com/juju/juju/state"
)

// isMachineWithJob returns whether the given entity is a machine that
// is configured to run the given job.
func isMachineWithJob(e state.Entity, j state.MachineJob) bool {
	m, ok := e.(*state.Machine)
	if !ok {
		return false
	}
	for _, mj := range m.Jobs() {
		if mj == j {
			return true
		}
	}
	return false
}

type validateArgs struct {
	statePool *state.StatePool
	modelUUID string
	// strict validation does not allow empty UUID values
	strict bool
	// controllerModelOnly only validates the controller model
	controllerModelOnly bool
}

// validateModelUUID is the common validator for the various
// apiserver components that need to check for a valid model
// UUID.  An empty modelUUID means that the connection has come in at
// the root of the URL space and refers to the controller
// model.
//
// It returns the validated model UUID.
func validateModelUUID(args validateArgs) (string, error) {
	overrideUUID, err := ensureValidModelUUID(args)
	if err != nil {
		return "", errors.Trace(err)
	}
	if overrideUUID != "" {
		return overrideUUID, nil
	}
	err = checkModelExists(args.statePool.SystemState(), args.modelUUID)
	if err != nil {
		return "", errors.Trace(err)
	}
	return args.modelUUID, nil
}

func ensureValidModelUUID(args validateArgs) (string, error) {
	ssState := args.statePool.SystemState()
	if args.modelUUID == "" {
		// We allow the modelUUID to be empty so that:
		//    TODO: server a limited API at the root (empty modelUUID)
		//    just the user manager and model manager are able to accept
		//    requests that don't require a modelUUID, like add-model.
		if args.strict {
			return "", errors.Trace(common.UnknownModelError(args.modelUUID))
		}
		return ssState.ModelUUID(), nil
	}
	if args.modelUUID == ssState.ModelUUID() {
		return ssState.ModelUUID(), nil
	}
	if args.controllerModelOnly {
		return "", errors.Unauthorizedf("requested model %q is not the controller model", args.modelUUID)
	}
	if !names.IsValidModel(args.modelUUID) {
		return "", errors.Trace(common.UnknownModelError(args.modelUUID))
	}
	return "", nil
}

func checkModelExists(ssState *state.State, modelUUID string) error {
	modelTag := names.NewModelTag(modelUUID)
	if _, err := ssState.GetModel(modelTag); err != nil {
		return errors.Wrap(err, common.UnknownModelError(modelUUID))
	}
	return nil
}

func getMigrationRedirectInfo(ssState *state.State, modelUUID string) (*params.RedirectInfoResult, error) {
	modelTag := names.NewModelTag(modelUUID)
	migration, err := ssState.LatestMigrationFor(modelTag)
	if err != nil && !errors.IsNotFound(err) {
		return nil, errors.Trace(err)
	}
	if migration != nil {
		phase, err := migration.Phase()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if phase.HasSucceeded() {
			return redirectInfoFromMigration(migration)
		}
	}
	return nil, nil
}

func redirectInfoFromMigration(migration state.ModelMigration) (*params.RedirectInfoResult, error) {
	target, err := migration.TargetInfo()
	if err != nil {
		return nil, errors.Trace(err)
	}
	netHostPorts, err := network.ParseHostPorts(target.Addrs...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	result := params.RedirectInfoResult{
		Servers: [][]params.HostPort{params.FromNetworkHostPorts(netHostPorts)},
		CACert:  target.CACert,
	}
	return &result, nil
}
