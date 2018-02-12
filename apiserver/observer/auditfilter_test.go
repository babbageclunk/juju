// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package observer_test

import (
	"strings"

	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils/set"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/apiserver/observer"
	apitesting "github.com/juju/juju/apiserver/testing"
	"github.com/juju/juju/core/auditlog"
)

type auditFilterSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&auditFilterSuite{})

func (s *auditFilterSuite) TestFiltersUninterestingConversations(c *gc.C) {
	target := &apitesting.FakeAuditLog{}
	filter := func(r auditlog.Request) bool {
		return !strings.HasPrefix(r.Method, "List")
	}
	log := observer.NewAuditLogFilter(target, filter)

	err := log.AddConversation(auditlog.Conversation{})
	c.Assert(err, jc.ErrorIsNil)
	// Nothing written out yet.
	target.CheckCallNames(c)

	err = log.AddRequest(auditlog.Request{RequestID: 1, Method: "ListBuckets"})
	c.Assert(err, jc.ErrorIsNil)
	target.CheckCallNames(c)

	err = log.AddResponse(auditlog.ResponseErrors{RequestID: 1})
	c.Assert(err, jc.ErrorIsNil)
	target.CheckCallNames(c)

	err = log.AddRequest(auditlog.Request{RequestID: 2, Method: "ListSpades"})
	c.Assert(err, jc.ErrorIsNil)
	target.CheckCallNames(c)

	err = log.AddResponse(auditlog.ResponseErrors{RequestID: 2})
	c.Assert(err, jc.ErrorIsNil)
	target.CheckCallNames(c)

	err = log.AddRequest(auditlog.Request{RequestID: 3, Method: "BuildCastle"})
	c.Assert(err, jc.ErrorIsNil)
	// Everything gets written now.
	target.CheckCallNames(c,
		"AddConversation", "AddRequest")
	calls := target.Calls()
	getMethod := func(i int) string {
		return calls[i].Args[0].(auditlog.Request).Method
	}
	requests := []string{getMethod(1)}
	c.Assert(requests, gc.DeepEquals, []string{"BuildCastle"})

	err = log.AddResponse(auditlog.ResponseErrors{RequestID: 3})
	c.Assert(err, jc.ErrorIsNil)
	target.CheckCallNames(c, "AddConversation", "AddRequest", "AddResponse")
}

func (s *auditFilterSuite) TestMakeFilter(c *gc.C) {
	f1 := observer.MakeInterestingRequestFilter(set.NewStrings("Battery.Kinzie", "Helplessness.Blues"))
	c.Assert(f1(auditlog.Request{Facade: "Battery", Method: "Kinzie"}), jc.IsFalse)
	c.Assert(f1(auditlog.Request{Facade: "Helplessness", Method: "Blues"}), jc.IsFalse)
	c.Assert(f1(auditlog.Request{Facade: "The", Method: "Shrine"}), jc.IsTrue)
}

func (s *auditFilterSuite) TestExpandsReadonlyMethods(c *gc.C) {
	f1 := observer.MakeInterestingRequestFilter(set.NewStrings("ReadOnlyMethods", "Helplessness.Blues"))
	c.Assert(f1(auditlog.Request{Facade: "Helplessness", Method: "Blues"}), jc.IsFalse)
	c.Assert(f1(auditlog.Request{Facade: "Client", Method: "FullStatus"}), jc.IsFalse)
	c.Assert(f1(auditlog.Request{Facade: "Falcon", Method: "Heavy"}), jc.IsTrue)
}

func (s *auditFilterSuite) TestOnlyExcludeReadonlyMethodsIfWeShould(c *gc.C) {
	f1 := observer.MakeInterestingRequestFilter(set.NewStrings("Helplessness.Blues"))
	c.Assert(f1(auditlog.Request{Facade: "Helplessness", Method: "Blues"}), jc.IsFalse)
	// Doesn't allow the readonly methods unless they've included the special key.
	c.Assert(f1(auditlog.Request{Facade: "Client", Method: "FullStatus"}), jc.IsTrue)
}
