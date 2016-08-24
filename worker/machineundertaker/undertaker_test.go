// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package machineundertaker_test

import (
	"time"

	"github.com/juju/errors"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/juju/names.v2"
	"gopkg.in/tomb.v1"

	"github.com/juju/juju/environs"
	"github.com/juju/juju/network"
	coretesting "github.com/juju/juju/testing"
	"github.com/juju/juju/watcher"
	"github.com/juju/juju/worker"
	"github.com/juju/juju/worker/machineundertaker"
	"github.com/juju/juju/worker/workertest"
)

type undertakerSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&undertakerSuite{})

func (s *undertakerSuite) TestErrorWatching(c *gc.C) {
	api := s.makeAPI()
	api.SetErrors(errors.New("blam"))
	w, err := machineundertaker.NewWorker(api, &fakeEnviron{})
	c.Assert(err, jc.ErrorIsNil)

	diedReason := make(chan error)
	go func() {
		diedReason <- w.Wait()
	}()
	select {
	case <-time.After(coretesting.LongWait):
		c.Fatalf("timed out waiting for worker to die")
	case err = <-diedReason:
	}

	c.Check(err, gc.ErrorMatches, "blam")
	api.CheckCallNames(c, "WatchMachineRemovals")
}

func (s *undertakerSuite) TestErrorGettingRemovals(c *gc.C) {
	api := s.makeAPI()
	api.SetErrors(nil, errors.New("explodo"))
	w, err := machineundertaker.NewWorker(api, &fakeEnviron{})
	c.Assert(err, jc.ErrorIsNil)

	diedReason := make(chan error)
	go func() {
		diedReason <- w.Wait()
	}()
	select {
	case <-time.After(coretesting.LongWait):
		c.Fatalf("timed out waiting for worker to die")
	case err = <-diedReason:
	}

	c.Check(err, gc.ErrorMatches, "explodo")
	api.CheckCallNames(c, "WatchMachineRemovals", "AllMachineRemovals")
}

func (s *undertakerSuite) TestNoRemovals(c *gc.C) {
	api := s.makeAPI()
	w, err := machineundertaker.NewWorker(api, &fakeEnviron{})
	c.Assert(err, jc.ErrorIsNil)

	workertest.CleanKill(c, w)

}

func (s *undertakerSuite) makeAPI() *fakeAPI {
	return &fakeAPI{
		Stub:    &testing.Stub{},
		watcher: s.newMockNotifyWatcher(),
	}
}

func (s *undertakerSuite) newMockNotifyWatcher() *mockNotifyWatcher {
	m := &mockNotifyWatcher{
		changes: make(chan struct{}, 1),
	}
	go func() {
		defer m.tomb.Done()
		defer m.tomb.Kill(nil)
		<-m.tomb.Dying()
	}()
	s.AddCleanup(func(c *gc.C) {
		err := worker.Stop(m)
		c.Check(err, jc.ErrorIsNil)
	})
	m.Change()
	return m
}

type fakeEnviron struct {
	environs.NetworkingEnviron

	*testing.Stub
}

func (e *fakeEnviron) ReleaseContainerAddresses(interfaces []network.ProviderInterfaceInfo) error {
	e.Stub.AddCall("ReleaseContainerAddresses", interfaces)
	return e.Stub.NextErr()
}

type fakeNoNetworkingEnviron struct {
	environs.Environ
}

type fakeAPI struct {
	machineundertaker.Facade

	*testing.Stub
	watcher    *mockNotifyWatcher
	removals   []names.MachineTag
	interfaces map[names.MachineTag][]network.ProviderInterfaceInfo
}

func (a *fakeAPI) WatchMachineRemovals() (watcher.NotifyWatcher, error) {
	a.Stub.AddCall("WatchMachineRemovals")
	return a.watcher, a.Stub.NextErr()
}

func (a *fakeAPI) AllMachineRemovals() ([]names.MachineTag, error) {
	a.Stub.AddCall("AllMachineRemovals")
	return a.removals, a.Stub.NextErr()
}

func (a *fakeAPI) GetProviderInterfaceInfo(machine names.MachineTag) ([]network.ProviderInterfaceInfo, error) {
	a.Stub.AddCall("GetProviderInterfaceInfo", machine)
	return a.interfaces[machine], a.Stub.NextErr()
}

func (a *fakeAPI) CompleteRemoval(machine names.MachineTag) error {
	a.Stub.AddCall("CompleteRemoval", machine)
	return a.Stub.NextErr()
}

type mockNotifyWatcher struct {
	watcher.NotifyWatcher

	tomb    tomb.Tomb
	changes chan struct{}
}

func (m *mockNotifyWatcher) Kill() {
	m.tomb.Kill(nil)
}

func (m *mockNotifyWatcher) Wait() error {
	return m.tomb.Wait()
}

func (m *mockNotifyWatcher) Changes() watcher.NotifyChannel {
	return m.changes
}

func (m *mockNotifyWatcher) Change() {
	m.changes <- struct{}{}
}
