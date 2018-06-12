// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package apiserver

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/juju/errors"
	"github.com/juju/utils/clock"

	"github.com/juju/juju/core/raftlog"
)

// RaftGetter represents something through which we can get access to
// raft stores.
type RaftGetter interface {
	LogStore() <-chan raftlog.Store
}

// raftHandler is a simple HTTP handler that lets us add log messages
// into the raft cluster FSM and read them back.
type raftHandler struct {
	raftBox RaftGetter
	clock   clock.Clock
}

// ServeHTTP implements HTTP.Handler
func (h *raftHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	store, err := h.getStore()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch req.Method {
	case http.MethodGet:
		h.doGet(w, store)
	case http.MethodPut:
		h.doPut(w, req, store)
	}
}

func (h *raftHandler) getStore() (raftlog.Store, error) {
	select {
	case <-h.clock.After(50 * time.Millisecond):
		return nil, errors.New("raftlog store not ready yet")
	case store := <-h.raftBox.LogStore():
		return store, nil
	}
}

func (h *raftHandler) doGet(w http.ResponseWriter, store raftlog.Store) error {
	_, err := w.Write([]byte(fmt.Sprintf("total records: %d\n", store.Count())))
	if err != nil {
		return errors.Trace(err)
	}
	for _, line := range store.Logs() {
		_, err := w.Write(line)
		if err != nil {
			return errors.Trace(err)
		}
		fmt.Fprintf(w, "\n")
	}
	return nil
}

func (h *raftHandler) doPut(w http.ResponseWriter, req *http.Request, store raftlog.Store) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logger.Errorf("raftHandler: %s", err)
		http.Error(w, fmt.Sprintf("couldn't read body: %s", err), http.StatusBadRequest)
		return
	}

	err = store.Append(body)
	if err != nil {
		logger.Errorf("raftHandler: %s", err)
		http.Error(w, fmt.Sprintf("append error: %s", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "records: %d\n", store.Count())
}
