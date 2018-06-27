// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package leadership

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/core/leadership"
	apiserverhub "github.com/juju/juju/pubsub/apiserver"
)

const (
	// FacadeName is the string-representation of this API used both
	// to register the service, and for the client to resolve the
	// service endpoint.
	FacadeName = "LeadershipService"

	// MinLeaseRequest is the shortest duration for which we will accept
	// a leadership claim.
	MinLeaseRequest = 5 * time.Second

	// MaxLeaseRequest is the longest duration for which we will accept
	// a leadership claim.
	MaxLeaseRequest = 5 * time.Minute

	// forwardedClaimTimeout is the amount of time we'll wait for the
	// raft leader to respond to a forwarded leadership claim.
	forwardedClaimTimeout = 1 * time.Second
)

// NewLeadershipServiceFacade constructs a new LeadershipService and presents
// a signature that can be used for facade registration.
func NewLeadershipServiceFacade(context facade.Context) (LeadershipService, error) {
	return NewLeadershipService(
		context.State().LeadershipClaimer(),
		context.RaftGetter(),
		context.Auth(),
		context.Hub(),
		context.Resources(),
	)
}

// NewLeadershipService constructs a new LeadershipService.
func NewLeadershipService(
	claimer leadership.Claimer,
	raftGetter facade.RaftGetter,
	authorizer facade.Authorizer,
	hub facade.Hub,
	resources facade.Resources,
) (LeadershipService, error) {

	if !authorizer.AuthUnitAgent() && !authorizer.AuthApplicationAgent() {
		return nil, errors.Unauthorizedf("permission denied")
	}

	// For fowarded claim requests we need a unique request ID (across
	// machines in this Juju controller) that will be used to send the
	// response. This will be the machine-id, connection-id and an
	// incremented request count on this facade.
	machineID, err := extractResourceValue(resources, "machineID")
	if err != nil {
		return nil, errors.Trace(err)
	}
	connectionID, err := extractResourceValue(resources, "connectionID")
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &leadershipService{
		claimer:     claimer,
		raftGetter:  raftGetter,
		authorizer:  authorizer,
		hub:         hub,
		requestBase: fmt.Sprintf("%s-%s", machineID, connectionID),
	}, nil
}

// leadershipService implements the LeadershipService interface and
// is the concrete implementation of the API endpoint.
type leadershipService struct {
	raftGetter  facade.RaftGetter
	claimer     leadership.Claimer
	authorizer  facade.Authorizer
	hub         facade.Hub
	requestBase string
	requestID   uint64
}

// ClaimLeadership is part of the LeadershipService interface.
func (m *leadershipService) ClaimLeadership(args params.ClaimLeadershipBulkParams) (params.ClaimLeadershipBulkResults, error) {

	results := make([]params.ErrorResult, len(args.Params))
	for pIdx, p := range args.Params {

		result := &results[pIdx]
		applicationTag, unitTag, err := parseApplicationAndUnitTags(p.ApplicationTag, p.UnitTag)
		if err != nil {
			result.Error = common.ServerError(err)
			continue
		}
		duration := time.Duration(p.DurationSeconds * float64(time.Second))
		if duration > MaxLeaseRequest || duration < MinLeaseRequest {
			result.Error = common.ServerError(errors.New("invalid duration"))
			continue
		}

		// In the future, situations may arise wherein units will make
		// leadership claims for other units. For now, units can only
		// claim leadership for themselves, for their own service.
		authTag := m.authorizer.GetAuthTag()
		canClaim := false
		switch authTag.(type) {
		case names.UnitTag:
			canClaim = m.authorizer.AuthOwner(unitTag) && m.authMember(applicationTag)
		case names.ApplicationTag:
			canClaim = m.authorizer.AuthOwner(applicationTag)
		}
		if !canClaim {
			result.Error = common.ServerError(common.ErrPerm)
			continue
		}
		if err = m.claimer.ClaimLeadership(applicationTag.Id(), unitTag.Id(), duration); err != nil {
			result.Error = common.ServerError(err)
		}
	}

	return params.ClaimLeadershipBulkResults{results}, nil
}

// ClaimLeadershipRaft is part of the LeadershipService
// interface. It'll be used for benchmarking raft transactions.
// ClaimLeadership is part of the LeadershipService interface.
func (m *leadershipService) ClaimLeadershipRaft(args params.ClaimLeadershipBulkParams) (params.ClaimLeadershipBulkResults, error) {
	results := make([]params.ErrorResult, len(args.Params))
	for pIdx, p := range args.Params {

		result := &results[pIdx]
		applicationTag, unitTag, err := parseApplicationAndUnitTags(p.ApplicationTag, p.UnitTag)
		if err != nil {
			result.Error = common.ServerError(err)
			continue
		}
		duration := time.Duration(p.DurationSeconds * float64(time.Second))
		if duration > MaxLeaseRequest || duration < MinLeaseRequest {
			result.Error = common.ServerError(errors.New("invalid duration"))
			continue
		}

		// In the future, situations may arise wherein units will make
		// leadership claims for other units. For now, units can only
		// claim leadership for themselves, for their own service.
		authTag := m.authorizer.GetAuthTag()
		canClaim := false
		switch authTag.(type) {
		case names.UnitTag:
			canClaim = m.authorizer.AuthOwner(unitTag) && m.authMember(applicationTag)
		case names.ApplicationTag:
			canClaim = m.authorizer.AuthOwner(applicationTag)
		}
		if !canClaim {
			result.Error = common.ServerError(common.ErrPerm)
			continue
		}
		result.Error = m.forwardClaimToLeader(applicationTag, unitTag, p.DurationSeconds)
	}

	return params.ClaimLeadershipBulkResults{results}, nil
}

func (m *leadershipService) forwardClaimToLeader(
	applicationTag names.ApplicationTag, unitTag names.UnitTag, durationSeconds float64,
) *params.Error {
	// Construct a unique topic for the response to be sent to, based
	// on machine id, connection id and request counter.
	requestID := atomic.AddUint64(&m.requestID, 1)
	responseTopic := fmt.Sprintf("%s.%s-%d", apiserverhub.LeadershipRequestTopic,
		m.requestBase, requestID)
	request := apiserverhub.LeadershipClaimRequest{
		ResponseTopic:   responseTopic,
		ModelUUID:       m.authorizer.ConnectedModel(),
		Application:     applicationTag.Id(),
		Unit:            unitTag.Id(),
		DurationSeconds: durationSeconds,
	}

	responseChan := make(chan apiserverhub.LeadershipClaimResponse, 1)
	errChan := make(chan error)
	// Get ready for the response to come back.
	unsubscribe, err := m.hub.Subscribe(
		responseTopic,
		func(_ string, resp apiserverhub.LeadershipClaimResponse, err error) {
			if err != nil {
				errChan <- err
				return
			}
			responseChan <- resp
		},
	)
	if err != nil {
		return common.ServerError(err)
	}
	defer unsubscribe()

	_, err = m.hub.Publish(apiserverhub.LeadershipRequestTopic, request)
	if err != nil {
		return common.ServerError(err)
	}

	select {
	case <-time.After(forwardedClaimTimeout):
		return &params.Error{Code: params.CodeTryAgain, Message: "timed out waiting for response from raft leader"}
	case err := <-errChan:
		return common.ServerError(err)
	case response := <-responseChan:
		if !response.Success {
			return &params.Error{Code: response.Code, Message: response.Message}
		}
	}
	return nil
}

// BlockUntilLeadershipReleased implements the LeadershipService interface.
func (m *leadershipService) BlockUntilLeadershipReleased(ctx context.Context, applicationTag names.ApplicationTag) (params.ErrorResult, error) {
	authTag := m.authorizer.GetAuthTag()
	hasPerm := false
	switch authTag.(type) {
	case names.UnitTag:
		hasPerm = m.authMember(applicationTag)
	case names.ApplicationTag:
		hasPerm = m.authorizer.AuthOwner(applicationTag)
	}

	if !hasPerm {
		return params.ErrorResult{Error: common.ServerError(common.ErrPerm)}, nil
	}

	if err := m.claimer.BlockUntilLeadershipReleased(applicationTag.Id(), ctx.Done()); err != nil {
		return params.ErrorResult{Error: common.ServerError(err)}, nil
	}
	return params.ErrorResult{}, nil
}

func (m *leadershipService) authMember(applicationTag names.ApplicationTag) bool {
	ownerTag := m.authorizer.GetAuthTag()
	unitTag, ok := ownerTag.(names.UnitTag)
	if !ok {
		return false
	}
	unitId := unitTag.Id()
	requireAppName, err := names.UnitApplication(unitId)
	if err != nil {
		return false
	}
	return applicationTag.Id() == requireAppName
}

// parseApplicationAndUnitTags takes in string representations of application
// and unit tags and returns their corresponding tags.
func parseApplicationAndUnitTags(
	applicationTagString, unitTagString string,
) (
	names.ApplicationTag, names.UnitTag, error,
) {
	// TODO(fwereade) 2015-02-25 bug #1425506
	// These permissions errors are not appropriate -- there's no permission or
	// security issue in play here, because our tag format is public, and the
	// error only triggers when the strings fail to match that format.
	applicationTag, err := names.ParseApplicationTag(applicationTagString)
	if err != nil {
		return names.ApplicationTag{}, names.UnitTag{}, common.ErrPerm
	}

	unitTag, err := names.ParseUnitTag(unitTagString)
	if err != nil {
		return names.ApplicationTag{}, names.UnitTag{}, common.ErrPerm
	}

	return applicationTag, unitTag, nil
}

func extractResourceValue(resources facade.Resources, key string) (string, error) {
	res := resources.Get(key)
	strRes, ok := res.(common.StringResource)
	if !ok {
		if res == nil {
			strRes = ""
		} else {
			return "", errors.Errorf("invalid %s resource: %v", key, res)
		}
	}
	return strRes.String(), nil
}
