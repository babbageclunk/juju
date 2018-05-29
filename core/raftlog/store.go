// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package raftlog

import (
	"time"

	"github.com/hashicorp/raft"
)

const raftTimeout = 10 * time.Second

// Store allows reading and writing stored log messages, through a
// raft cluster.
type Store interface {
	// Append adds a message to the log.
	Append(data []byte) error

	// Logs returns all the current logs.
	Logs() [][]byte

	// Count returns how many logs we have.
	Count() int
}

// NewStore makes a Store from the raft and FSM passed in.
func NewStore(fsm *FSM, raft *raft.Raft) Store {
	return &store{fsm: fsm, raft: raft}
}

type store struct {
	fsm  *FSM
	raft *raft.Raft
}

// Append implements Store.
func (s *store) Append(data []byte) error {
	result := s.raft.Apply(data, raftTimeout)
	return result.Error()
}

// Logs implements Store.
func (s *store) Logs() [][]byte {
	return s.fsm.Logs()
}

func (s *store) Count() int {
	return s.fsm.logs.Len()
}
