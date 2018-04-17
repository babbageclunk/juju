// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package raftbox

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"gopkg.in/juju/worker.v1"

	"github.com/juju/juju/core/raftlog"
	"github.com/juju/juju/worker/catacomb"
)

var logger = loggo.GetLogger("juju.worker.raft.raftbox")

// New returns a new empty raft-box worker. This is needed to allow
// the API server to get the running raft node without depending on
// the raft worker directly. If API server depends on raft it
// yields a dependency loop with this structure: api-server ->
// raft-worker -> raft-transport -> peer-grouper (implicitly via
// published API details) -> upgrade-check-flag -> upgrader ->
// api-caller -> api-server (implicitly). To avoid this both the raft
// worker and the API server will depend on this worker; when raft is
// up it will put the *Raft in the box, and when the API server
// receives a request that requires raft it can get it from the box.
func New() (worker.Worker, error) {
	w := &box{
		storeIn:  make(chan raftlog.Store),
		storeOut: make(chan raftlog.Store),
	}
	if err := catacomb.Invoke(catacomb.Plan{
		Site: &w.catacomb,
		Work: w.loop,
	}); err != nil {
		return nil, errors.Trace(err)
	}
	return w, nil
}

type box struct {
	catacomb          catacomb.Catacomb
	storeIn, storeOut chan raftlog.Store
}

func (b *box) loop() error {
	var (
		out   chan raftlog.Store
		store raftlog.Store
	)
	for {
		select {
		case <-b.catacomb.Dying():
			return b.catacomb.ErrDying()
		case store = <-b.storeIn:
			logger.Debugf("new raftlog.Store put into the box")
			out = b.storeOut
		case out <- store:
		}
	}
}

// Kill implements worker.Worker.
func (b *box) Kill() {
	b.catacomb.Kill(nil)
}

// Wait implements worker.Worker.
func (b *box) Wait() error {
	return b.catacomb.Wait()
}

// LogStore returns a channel yielding the contained raftlog.Store (once it's set).
func (b *box) LogStore() <-chan raftlog.Store {
	return b.storeOut
}

// Put accepts a raft stores that will be returned to anyone calling the associated getter for that type.
func (b *box) Put(value interface{}) error {
	switch store := value.(type) {
	case raftlog.Store:
		b.storeIn <- store
	default:
		return errors.Errorf("unexpected store type %T, expected raftlog.Store", value)
	}
	return nil
}
