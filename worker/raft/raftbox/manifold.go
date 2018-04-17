// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package raftbox

import (
	"github.com/juju/errors"
	worker "gopkg.in/juju/worker.v1"

	"github.com/juju/juju/core/raftlog"
	"github.com/juju/juju/worker/dependency"
)

// ManifoldConfig holds the information needed to run a raftbox worker
// in a dependency.Engine.
type ManifoldConfig struct {
	NewWorker func() (worker.Worker, error)
}

func (config ManifoldConfig) start(context dependency.Context) (worker.Worker, error) {
	w, err := config.NewWorker()
	if err != nil {
		return nil, errors.Trace(err)
	}
	return w, err
}

// Manifold returns a dependency.Manifold for running a raft box and
// exposing it as output.
func Manifold(config ManifoldConfig) dependency.Manifold {
	return dependency.Manifold{
		Start:  config.start,
		Output: boxOutput,
	}
}

// Putter enables a client to put a raft store into the box.
type Putter interface {
	Put(interface{}) error
}

// Getter enables the client to get raft stores from the box.
type Getter interface {
	LogStore() <-chan raftlog.Store
}

func boxOutput(in worker.Worker, out interface{}) error {
	b, ok := in.(*box)
	if !ok {
		return errors.Errorf("expected *box, got %T", in)
	}
	switch target := out.(type) {
	case *Putter:
		*target = b
	case *Getter:
		*target = b
	default:
		return errors.Errorf("expected Putter or Getter, got %T")
	}
	return nil
}
