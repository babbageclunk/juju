// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package apiserver

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/hashicorp/raft"
	"github.com/juju/utils/clock"
)

// RaftGetter represents something through which we can get access to
// the raft node
type RaftGetter interface {
	Get() <-chan *raft.Raft
}

// raftHandler is a simple HTTP handler that lets us add log messages
// into the raft cluster FSM and read them back.
type raftHandler struct {
	raftBox RaftGetter
	clock   clock.Clock
}

// ServeHTTP implements HTTP.Handler
func (h *raftHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		// punting on this for now - would need the fsm to be passed around too.
	case http.MethodPut:
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("couldn't read body: %s", err), http.StatusBadRequest)
			return
		}

		var r *raft.Raft
		select {
		case <-h.clock.After(50 * time.Millisecond):
			http.Error(w, "raft not ready yet", http.StatusInternalServerError)
			return
		case r = <-h.raftBox.Get():
		}

		result := r.Apply(body, 0)
		err = result.Error()
		if err != nil {
			http.Error(w, fmt.Sprintf("apply error: %s", err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "records: %d, index: %d\n", result.Response(), result.Index())
	}
}
