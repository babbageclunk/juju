// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"sync"

	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"
	"gopkg.in/tomb.v1"
)

// NewStatePool returns a new StatePool instance. It takes a State
// connected to the system (controller model).
func NewStatePool(systemState *State, tomb *tomb.Tomb) *StatePool {
	result := StatePool{
		systemState: systemState,
		tomb:        tomb,
		pool:        make(map[string]*PoolItem),
	}
	result.tomb.Go(result.watchModels)
	return &result
}

// StatePool is a simple cache of State instances for multiple models.
type StatePool struct {
	systemState *State
	tomb        *tomb.Tomb
	// mu protects pool
	mu   sync.Mutex
	pool map[string]*PoolItem
}

type PoolItem struct {
	state   *State
	refs    uint
	removed bool
}

// Get returns a State for a given model from the pool, creating
// one if required.
func (p *StatePool) Get(modelUUID string) (*State, error) {
	if modelUUID == p.systemState.ModelUUID() {
		return p.systemState, nil
	}

	logger.Debugf("getting state from pool")
	p.mu.Lock()
	defer p.mu.Unlock()

	item, ok := p.pool[modelUUID]
	if ok && item.removed {
		// We don't want to allow increasing the refcount of a model
		// that's been removed.
		return nil, errors.Errorf("model %v has been removed", modelUUID)
	}
	if ok {
		item.refs++
		return item.state, nil
	}

	st, err := p.systemState.ForModel(names.NewModelTag(modelUUID))
	if err != nil {
		return nil, errors.Annotatef(err, "failed to create state for model %v", modelUUID)
	}
	p.pool[modelUUID] = &PoolItem{state: st, refs: 1}
	return st, nil
}

// Put indicates that the client has finished using the State.
func (p *StatePool) Put(modelUUID string) error {
	if modelUUID == p.systemState.ModelUUID() {
		// We don't maintain a refcount for the controller.
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	item, ok := p.pool[modelUUID]
	if !ok {
		return errors.Errorf("unable to return unknown model %v to the pool", modelUUID)
	}
	if item.refs == 0 {
		return errors.Errorf("state pool refcount for model %v is already 0", modelUUID)
	}
	item.refs--
	p.maybeRemoveItem(modelUUID, item)
	return nil
}

// SystemState returns the State passed in to NewStatePool.
func (p *StatePool) SystemState() *State {
	return p.systemState
}

// Close closes all State instances in the pool.
func (p *StatePool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	for _, item := range p.pool {
		err := item.state.Close()
		if err != nil {
			lastErr = err
		}
	}
	p.pool = make(map[string]*PoolItem)
	return errors.Annotate(lastErr, "at least one error closing a state")
}

func (p *StatePool) maybeRemoveItem(modelUUID string, item *PoolItem) {
	if item.removed && item.refs == 0 {
		delete(p.pool, modelUUID)
	}
}

func (p *StatePool) watchModels() error {
	watcher := p.systemState.WatchModels()
	for {
		select {
		case changes := <-watcher.Changes():
			logger.Debugf("statepool saw model changes: %v", changes)
		case <-p.tomb.Dying():
			return tomb.ErrDying
		}
	}
}
