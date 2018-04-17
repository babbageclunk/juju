// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package raftbox

import (
	"github.com/hashicorp/raft"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"gopkg.in/juju/worker.v1"

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
		raftIn:  make(chan *raft.Raft),
		raftOut: make(chan *raft.Raft),
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
	catacomb        catacomb.Catacomb
	raftIn, raftOut chan *raft.Raft
}

func (b *box) loop() error {
	var (
		out chan *raft.Raft
		r   *raft.Raft
	)
	for {
		select {
		case <-b.catacomb.Dying():
			return b.catacomb.ErrDying()
		case r = <-b.raftIn:
			logger.Debugf("new raft put into the box")
			out = b.raftOut
		case out <- r:
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

// Get returns a channel yielding the contained raft (once it's set).
func (b *box) Get() <-chan *raft.Raft {
	return b.raftOut
}

// Put accepts a raft that will be returned to anyone calling Get.
func (b *box) Put(r *raft.Raft) {
	b.raftIn <- r
}
