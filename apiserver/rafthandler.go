// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package apiserver

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/hashicorp/raft"
)

// raftHandler is a simple HTTP handler that lets us add log messages
// into the raft cluster FSM and read them back.
type raftHandler struct {
	raft *raft.Raft
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
		result := h.raft.Apply(body, 0)
		err = result.Error()
		if err != nil {
			http.Error(w, fmt.Sprintf("apply error: %s", err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "records: %d, index: %d", result.Response(), result.Index())
	}
}
