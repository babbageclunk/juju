// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package crasher

import (
	"github.com/juju/errors"
	"gopkg.in/juju/worker.v1"
	"gopkg.in/juju/worker.v1/catacomb"
	"gopkg.in/juju/worker.v1/dependency"
)

// ManifoldConfig holds everything needed to run a crasher.
type ManifoldConfig struct {
	Name string
}

func (config ManifoldConfig) start(context dependency.Context) (worker.Worker, error) {
	return newWorker(config.Name)
}

// Manifold creates a dependency.Manifold thatw will start a crasher.
func Manifold(config ManifoldConfig) dependency.Manifold {
	return dependency.Manifold{
		Inputs: nil,
		Start:  config.start,
	}
}

type crasher struct {
	catacomb catacomb.Catacomb
	name     string
}

func newWorker(name string) (*crasher, error) {
	w := &crasher{name: name}
	if err := catacomb.Invoke(catacomb.Plan{
		Site: &w.catacomb,
		Work: w.loop,
	}); err != nil {
		return nil, errors.Trace(err)
	}
	return w, nil
}

func (w *crasher) Kill() {
	w.catacomb.Kill(nil)
}

func (w *crasher) Wait() error {
	return w.catacomb.Wait()
}

func (w *crasher) loop() error {
	return errors.Errorf("oops, %s crashed!", w.name)
}
