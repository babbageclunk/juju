// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package raftlease

import (
	"bytes"
	"encoding/gob"
	"io"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	"github.com/juju/errors"

	"github.com/juju/juju/core/lease"
)

// FSM manages a set of leases as a raft.FSM.
type FSM struct {
	mu sync.Mutex

	entries map[string]entry
}

// Apply implements raft.FSM.
func (f *FSM) Apply(log *raft.Log) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()

	var req request
	if err := gob.NewDecoder(bytes.NewReader(log.Data)).Decode(&req); err != nil {
		return err
	}
	switch req.Operation {
	case LeaseClaim:
		return f.claim(req)
	case LeaseExtend:
		return f.extend(req)
	case LeaseExpire:
		return f.expire(req)
	default:
		return errors.Errorf("unknown lease operation %q", req.Operation)
	}
}

func (f *FSM) claim(req request) error {
	return lease.ErrInvalid
}

func (f *FSM) extend(req request) error {
	return lease.ErrInvalid
}

func (f *FSM) expire(req request) error {
	return lease.ErrInvalid
}

// Snapshot implements raft.FSM.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data := make(map[string]entry)
	for key := range f.entries {
		data[key] = f.entries[key]
	}
	return &Snapshot{entries: data}, nil
}

// Restore implements raft.FSM.
func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	var entries map[string]entry
	if err := gob.NewDecoder(rc).Decode(&entries); err != nil {
		return err
	}
	f.mu.Lock()
	f.entries = entries
	f.mu.Unlock()
	return nil
}

// LeaseOperation enumerates the possible update operations that can
// be made to leases.
type LeaseOperation string

const (
	// LeaseClaim records the holder's claim to the specified
	// lease. If it succeeds, the claim is guaranteed until at least
	// the supplied duration after the claim was
	// initiated.
	LeaseClaim LeaseOperation = "claim"

	// LeaseExtend records the holders continued claim to the lease
	// that they previously held. If it succeeds, the claim is
	// guaranteed until at least the supplied duration after the
	// extend was initiated.
	LeaseExtend LeaseOperation = "extend"

	// LeaseExpire records the vacation of the supplied lease. It will
	// fail if we cannot verify that the expiry time has passed.
	LeaseExpire LeaseOperation = "expire"
)

// request describes an operation to be done on a lease.
type request struct {
	Operation LeaseOperation
	Lease     string
	// TODO(babbageclunk): clock skew - between machines, and within a
	// machine if the VM's time is updated. We can't rely on monotonic
	// clocks since these commands will be written to disk and sent
	// over the wire.
	At       time.Time
	Holder   string
	Duration time.Duration
}

// Snapshot stores the state of all the leases at a point in time. It
// implements raft.FSMSnapshot.
// TODO(babbageclunk): versioning - if we change the shape of entries
// or the commands, how do we handle that?
type Snapshot struct {
	entries map[string]entry
}

// Persist implements raft.FSMSnapshot.
func (snap *Snapshot) Persist(sink raft.SnapshotSink) error {
	if err := gob.NewEncoder(sink).Encode(snap.entries); err != nil {
		sink.Cancel()
		return err
	}
	sink.Close()
	return nil
}

// Release implements raft.FSMSnapshot.
func (snap *Snapshot) Release() {}

// entry holds the details of a lease and how it was written.
type entry struct {
	// holder identifies the current holder of the lease.
	holder string

	// start is the global time at which the lease started.
	start time.Time

	// duration is the duration for which the lease is valid,
	// from the start time.
	duration time.Duration

	// epoch is a number that is incremented for the lease each time
	// the holder is changed. It can be used to track
	writer string
}
