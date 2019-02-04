// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package lease_test

import (
	"sync"
	"time"

	"github.com/juju/clock"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	corelease "github.com/juju/juju/core/lease"
	"github.com/juju/juju/worker/lease"
)

type perfSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&perfSuite{})

func (s *perfSuite) TestClaims(c *gc.C) {
	store := newLeaseStore(clock.WallClock, &nullTarget{}, nullTrapdoor)
	manager, err := lease.NewManager(lease.ManagerConfig{
		Clock: clock.WallClock,
		Store: store,
		Secretary: func(string) (lease.Secretary, error) {
			return Secretary{}, nil
		},
		MaxSleep:             defaultMaxSleep,
		Logger:               loggo.GetLogger("lease_test"),
		PrometheusRegisterer: noopRegisterer{},
	})
	c.Assert(err, jc.ErrorIsNil)

	// Make 1000 applications with 5 workers each.
	var (
		ready, stopped sync.WaitGroup
	)

	start := make(chan struct{})
	stop := make(chan struct{})
	samples := make(chan sample)

	claimer, err := manager.Claimer("leadership", "a-model")
	c.Assert(err, jc.ErrorIsNil)

	for app := 0; app < 1000; app++ {
		lease := fmt.Sprintf("app-%s", app)
		for unit := 0; unit < 5; unit++ {
			name := fmt.Sprintf("unit-%s-%s", app, unit)
			w := &worker{
				start:   start,
				stop:    stop,
				claimer: claimer,
				lease:   lease,
				name:    name,
				samples: samples,
			}
			ready.Add(1)
			stopped.Add(1)
			go w.loop(&ready, &stopped)
		}
	}

	// Wait for all of the goroutines to be ready to go.
	ready.Wait()
	close(start)

}

var (
	claimDuration = 10 * time.Second
	reclaimSleep  = 5 * time.Second
)

type sample struct {
	operation string
	value     time.Duration
}

type worker struct {
	start   <-chan struct{}
	stop    <-chan struct{}
	claimer corelease.Claimer
	lease   string
	name    string
	samples chan<- sample
}

func (w *worker) run(ready, stopped *sync.WaitGroup) {
	readyGroup.Done()
	defer stopped.Done()
	<-start
	for {
		startTime := time.Now()
		err := w.claimer.Claim(w.lease, w.name, claimDuration)
		endTime := time.Now()
		if err == corelease.ErrClaimDenied {
			w.record("claim-failed", startTime, endTime)

			err := w.claimer.WaitUntilExpired(w.lease, w.stop)
			if err == corelease.ErrWaitCancelled {
				return
			} else if err != nil {
				panic(err)
			}
			continue
		} else if err != nil {
			panic(err)
		} else {
			w.record("claim-succeeded", startTime, endTime)
			for {
				select {
				case <-stop:
					return
				case <-time.After(reclaimSleep):
					startTime = time.Now()
					err := w.claimer.Claim(w.lease, w.name, claimDuration)
					endTime = time.Now()
					if err == corelease.ErrClaimDenied {
						panic("lost lease")
					} else if err != nil {
						panic(err)
					}
					w.record("extend-succeeded", startTime, endTime)
				}
			}
		}
	}
}

func (w *worker) record(event string, start, end time.Time) {
	s := sample{event, end.Sub(start)}
	select {
	case <-w.stop:
	case w.samples <- s:
	}
}

// leaseStore implements lease.Store as simply as possible for use in
// the dummy provider. Heavily cribbed from raftlease.FSM.
type leaseStore struct {
	mu       sync.Mutex
	clock    clock.Clock
	entries  map[lease.Key]*entry
	trapdoor raftlease.TrapdoorFunc
	target   raftlease.NotifyTarget
}

// entry holds the details of a lease.
type entry struct {
	// holder identifies the current holder of the lease.
	holder string

	// start is the global time at which the lease started.
	start time.Time

	// duration is the duration for which the lease is valid,
	// from the start time.
	duration time.Duration
}

func newLeaseStore(clock clock.Clock, target raftlease.NotifyTarget, trapdoor raftlease.TrapdoorFunc) *leaseStore {
	return &leaseStore{
		clock:    clock,
		entries:  make(map[lease.Key]*entry),
		target:   target,
		trapdoor: trapdoor,
	}
}

// Autoexpire is part of lease.Store.
func (*leaseStore) Autoexpire() bool { return false }

// ClaimLease is part of lease.Store.
func (s *leaseStore) ClaimLease(key lease.Key, req lease.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, found := s.entries[key]; found {
		return lease.ErrInvalid
	}
	s.entries[key] = &entry{
		holder:   req.Holder,
		start:    s.clock.Now(),
		duration: req.Duration,
	}
	s.target.Claimed(key, req.Holder)
	return nil
}

// ExtendLease is part of lease.Store.
func (s *leaseStore) ExtendLease(key lease.Key, req lease.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, found := s.entries[key]
	if !found {
		return lease.ErrInvalid
	}
	if entry.holder != req.Holder {
		return lease.ErrInvalid
	}
	now := s.clock.Now()
	expiry := now.Add(req.Duration)
	if !expiry.After(entry.start.Add(entry.duration)) {
		// No extension needed - the lease already expires after the
		// new time.
		return nil
	}
	// entry is a pointer back into the f.entries map, so this update
	// isn't lost.
	entry.start = now
	entry.duration = req.Duration
	return nil
}

// Expire is part of lease.Store.
func (s *leaseStore) ExpireLease(key lease.Key) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, found := s.entries[key]
	if !found {
		return lease.ErrInvalid
	}
	expiry := entry.start.Add(entry.duration)
	if !s.clock.Now().After(expiry) {
		return lease.ErrInvalid
	}
	delete(s.entries, key)
	s.target.Expired(key)
	return nil
}

// Leases is part of lease.Store.
func (s *leaseStore) Leases(keys ...lease.Key) map[lease.Key]lease.Info {
	s.mu.Lock()
	defer s.mu.Unlock()

	filter := make(map[lease.Key]bool)
	filtering := len(keys) > 0
	if filtering {
		for _, key := range keys {
			filter[key] = true
		}
	}

	results := make(map[lease.Key]lease.Info)
	for key, entry := range s.entries {
		if filtering && !filter[key] {
			continue
		}

		results[key] = lease.Info{
			Holder:   entry.holder,
			Expiry:   entry.start.Add(entry.duration),
			Trapdoor: s.trapdoor(key, entry.holder),
		}
	}
	return results
}

// Refresh is part of lease.Store.
func (s *leaseStore) Refresh() error {
	return nil
}

// PinLease is part of lease.Store.
func (s *leaseStore) PinLease(key lease.Key, entity string) error {
	return errors.NotImplementedf("lease pinning")
}

// UnpinLease is part of lease.Store.
func (s *leaseStore) UnpinLease(key lease.Key, entity string) error {
	return errors.NotImplementedf("lease unpinning")
}

// Pinned is part of the Store interface.
func (s *leaseStore) Pinned() map[lease.Key][]string {
	return nil
}

type nullTarget struct{}

func (*nullTarget) Claimed(corelease.Key, string) {}
func (*nullTarget) Expired(corelease.Key)         {}

func nullTrapdoor(corelease.Key, string) corelease.Trapdoor {
	return nil
}
