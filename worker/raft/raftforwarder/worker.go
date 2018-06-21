// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package raftforwarder

import (
	"time"

	"github.com/hashicorp/raft"
	"github.com/juju/errors"
	"github.com/juju/pubsub"
	"gopkg.in/juju/worker.v1"
	"gopkg.in/yaml.v2"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/pubsub/apiserver"
	"github.com/juju/juju/worker/catacomb"
)

const applyTimeout = 5 * time.Second

// This worker receives leadership claims forwarded from API servers
// running on non-leader machines and applies them.

// RaftApplier allows applying a command to the raft FSM.
type RaftApplier interface {
	Apply(cmd []byte, timeout time.Duration) raft.ApplyFuture
}

// Logger specifies the interface we use from loggo.Logger.
type Logger interface {
	Tracef(string, ...interface{})
}

// Config defines the resources the worker needs to run.
type Config struct {
	Hub    *pubsub.StructuredHub
	Raft   RaftApplier
	Logger Logger
}

// Validate checks that this config can be used.
func (config Config) Validate() error {
	if config.Hub == nil {
		return errors.NotValidf("nil Hub")
	}
	if config.Raft == nil {
		return errors.NotValidf("nil Raft")
	}
	if config.Logger == nil {
		return errors.NotValidf("nil Logger")
	}
	return nil
}

// NewWorker creates and starts a worker that will forward leadership
// claims from non-raft-leader machines.
func NewWorker(config Config) (worker.Worker, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}
	w := &forwarder{
		config: config,
	}
	if err := catacomb.Invoke(catacomb.Plan{
		Site: &w.catacomb,
		Work: w.loop,
	}); err != nil {
		return nil, errors.Trace(err)
	}
	return w, nil
}

type forwarder struct {
	catacomb catacomb.Catacomb
	config   Config
}

// Kill is part of the worker.Worker interface.
func (w *forwarder) Kill() {
	w.catacomb.Kill(nil)
}

// Wait is part of the worker.Worker interface.
func (w *forwarder) Wait() error {
	return w.catacomb.Wait()
}

func (w *forwarder) loop() error {
	unsubscribe, err := w.config.Hub.Subscribe(apiserver.LeadershipRequestTopic, w.handleRequest)
	if err != nil {
		return errors.Trace(err)
	}
	defer unsubscribe()
	<-w.catacomb.Dying()
	return w.catacomb.ErrDying()
}

func (w *forwarder) handleRequest(_ string, req apiserver.LeadershipClaimRequest, err error) {
	w.config.Logger.Tracef("received %#v, err: %s", req, err)
	if err != nil {
		// This should never happen, so treat it as fatal.
		w.catacomb.Kill(errors.Annotate(err, "requests callback failed"))
		return
	}
	response := responseFromError(w.processRequest(req))
	_, err = w.config.Hub.Publish(req.ResponseTopic, response)
	if err != nil {
		w.catacomb.Kill(errors.Annotate(err, "publishing response"))
		return
	}
}

func (w *forwarder) processRequest(req apiserver.LeadershipClaimRequest) error {
	// For now just serialise the message and treat that as the FSM command.
	cmd, err := yaml.Marshal(req)
	if err != nil {
		return errors.Annotate(err, "serialising command")
	}
	future := w.config.Raft.Apply(cmd, applyTimeout)
	if err = future.Error(); err != nil {
		return errors.Annotate(err, "applying command")
	}
	// Response will always be an error or nil, but not in the case of
	// the current FSM.
	// respValue := future.Response()
	// responseErr, ok := respValue.(error)
	// if respValue != nil && !ok {
	// 	return errors.Errorf("FSM response must be an error or nil, got %#v", respValue)
	// }
	// return responseErr
	return nil
}

func responseFromError(err error) apiserver.LeadershipClaimResponse {
	var response apiserver.LeadershipClaimResponse
	if err == nil {
		response.Success = true
	} else {
		paramsErr := common.ServerError(err)
		response.Message = paramsErr.Message
		response.Code = paramsErr.Code
	}
	return response
}
