// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package raftlog

import (
	"encoding/gob"
	"io"
	"sync"

	"github.com/hashicorp/raft"
	"github.com/juju/collections/deque"
)

// FSM is an implementation of raft.FSM, which simply appends
// log data to a slice.
type FSM struct {
	mu   sync.Mutex
	logs *deque.Deque
}

// NewFSM creates a new empty FSM.
func NewFSM() *FSM {
	return &FSM{logs: deque.NewWithMaxLen(1000)}
}

// Logs returns the accumulated log data.
func (fsm *FSM) Logs() [][]byte {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()
	copied := make([][]byte, fsm.logs.Len())
	for i := 0; i < fsm.logs.Len(); i++ {
		item, _ := fsm.logs.PopFront()
		copied = append(copied, item.([]byte))
		fsm.logs.PushBack(item)
	}
	return copied
}

// Apply is part of the raft.FSM interface.
func (fsm *FSM) Apply(log *raft.Log) interface{} {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()
	fsm.logs.PushBack(log.Data)

	return fsm.logs.Len()
}

// Snapshot is part of the raft.FSM interface.
func (fsm *FSM) Snapshot() (raft.FSMSnapshot, error) {
	copied := fsm.Logs()
	return &SimpleSnapshot{copied, len(copied)}, nil
}

// Restore is part of the raft.FSM interface.
func (fsm *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	var logs [][]byte
	if err := gob.NewDecoder(rc).Decode(&logs); err != nil {
		return err
	}
	fsm.mu.Lock()
	for _, item := range logs {
		fsm.logs.PushBack(item)
	}
	fsm.mu.Unlock()
	return nil
}

// SimpleSnapshot is an implementation of raft.FSMSnapshot, returned
// by the FSM.Snapshot in this package.
type SimpleSnapshot struct {
	logs [][]byte
	n    int
}

// Persist is part of the raft.FSMSnapshot interface.
func (snap *SimpleSnapshot) Persist(sink raft.SnapshotSink) error {
	if err := gob.NewEncoder(sink).Encode(snap.logs[:snap.n]); err != nil {
		sink.Cancel()
		return err
	}
	sink.Close()
	return nil
}

// Release is part of the raft.FSMSnapshot interface.
func (*SimpleSnapshot) Release() {}
