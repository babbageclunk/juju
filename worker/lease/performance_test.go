// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package lease_test

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	corelease "github.com/juju/juju/core/lease"
	"github.com/juju/juju/core/raftlease"
	"github.com/juju/juju/worker/lease"
)

type perfSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&perfSuite{})

var logger = loggo.GetLogger("xtian.perf")

func (s *perfSuite) TestClaims(c *gc.C) {
	store := newLeaseStore(clock.WallClock, &nullTarget{}, nullTrapdoor)
	manager, err := lease.NewManager(lease.ManagerConfig{
		Clock: clock.WallClock,
		Store: store,
		Secretary: func(string) (lease.Secretary, error) {
			return &nullSecretary{}, nil
		},
		MaxSleep:             defaultMaxSleep,
		Logger:               logger,
		PrometheusRegisterer: noopRegisterer{},
	})
	c.Assert(err, jc.ErrorIsNil)

	// Make 1000 applications with 5 workers each.
	var (
		ready, stopped sync.WaitGroup
	)

	start := make(chan struct{})
	stop := make(chan struct{})
	defer close(stop)

	claimer, err := manager.Claimer("leadership", "a-model")
	c.Assert(err, jc.ErrorIsNil)

	c.Logf("starting collector")
	coll := newCollector(maxSamples, stop)
	ready.Add(1)
	stopped.Add(1)
	go coll.run(&ready, &stopped)

	c.Logf("starting applications")
	for app := 0; app < totalApps; app++ {
		lease := fmt.Sprintf("app-%d", app)
		for unit := 0; unit < unitsPerApp; unit++ {
			name := fmt.Sprintf("unit-%d-%d", app, unit)
			w := &leaseGrabber{
				start:   start,
				stop:    coll.stopWorkers,
				claimer: claimer,
				lease:   lease,
				name:    name,
				samples: coll.samples,
			}
			ready.Add(1)
			stopped.Add(1)
			go w.run(&ready, &stopped)
		}
	}

	c.Logf("waiting for all goroutines to be ready")
	ready.Wait()

	c.Logf("starting workers")
	close(start)

	stopped.Wait()
	manager.Kill()
	err = manager.Wait()
	c.Assert(err, jc.ErrorIsNil)

	c.Logf(coll.results())
}

var (
	claimDuration = 10 * time.Second
	reclaimSleep  = 5 * time.Second
	totalApps     = 500
	unitsPerApp   = 5
	maxSamples    = 10000
)

type sample struct {
	operation string
	value     time.Duration
}

type leaseGrabber struct {
	start   <-chan struct{}
	stop    <-chan struct{}
	claimer corelease.Claimer
	lease   string
	name    string
	samples chan<- sample
}

func (w *leaseGrabber) run(ready, stopped *sync.WaitGroup) {
	ready.Done()
	defer stopped.Done()
	<-w.start
	state := "candidate"
	for {
		switch state {
		case "candidate":
			startTime := time.Now()
			err := w.claimer.Claim(w.lease, w.name, claimDuration)
			endTime := time.Now()
			if err == corelease.ErrClaimDenied {
				w.record("claim-failed", startTime, endTime)
				state = "follower"
				continue
			} else if err != nil {
				panic(err)
			}
			w.record("claim-succeeded", startTime, endTime)
			state = "leader"
		case "leader":
			select {
			case <-w.stop:
				return
			case <-time.After(reclaimSleep):
				startTime := time.Now()
				err := w.claimer.Claim(w.lease, w.name, claimDuration)
				endTime := time.Now()
				if err == corelease.ErrClaimDenied {
					w.record("extend-failed", startTime, endTime)
					state = "follower"
					continue
				} else if err != nil {
					panic(err)
				}
				state = "leader"
				w.record("extend-succeeded", startTime, endTime)
			}
		case "follower":
			err := w.claimer.WaitUntilExpired(w.lease, w.stop)
			if err == corelease.ErrWaitCancelled {
				return
			} else if err != nil {
				panic(err)
			}
			state = "candidate"
		}
	}
}

func (w *leaseGrabber) record(event string, start, end time.Time) {
	s := sample{event, end.Sub(start)}
	select {
	case <-w.stop:
	case w.samples <- s:
	}
}

type totals struct {
	count int
	max   time.Duration
	min   time.Duration
	mean  float64
	m2    float64
}

func (t *totals) update(val time.Duration) {
	t.count++
	if val > t.max {
		t.max = val
	}
	if val < t.min {
		t.min = val
	}
	floatVal := float64(val)
	// From Welford's algorithm for calculating online variance.
	// https://en.wikipedia.org/wiki/Algorithms_for_calculating_variance#Welford's_Online_algorithm
	delta := floatVal - t.mean
	t.mean += delta / float64(t.count)
	delta2 := floatVal - t.mean
	t.m2 += delta * delta2
}

func (t *totals) String() string {
	return fmt.Sprintf("%8d\t%8s\t%14s\t%14s\t%14s",
		t.count, t.min, t.max, time.Duration(t.mean), time.Duration(t.stddev()),
	)
}

func (t totals) stddev() float64 {
	if t.count == 0 {
		return 0
	}
	return math.Sqrt(t.m2 / float64(t.count))
}

func newCollector(maxSamples int, abort chan struct{}) *collector {
	return &collector{
		maxSamples:  maxSamples,
		samples:     make(chan sample, maxSamples),
		stopWorkers: make(chan struct{}),
		abort:       abort,
		data: map[string]*totals{
			"claim-failed":     {},
			"claim-succeeded":  {},
			"extend-failed":    {},
			"extend-succeeded": {},
		},
	}
}

type collector struct {
	maxSamples  int
	sampleCount int
	samples     chan sample
	// We close this to stop the worker goroutines.
	stopWorkers chan struct{}
	// If someone closes this we stop.
	abort chan struct{}
	data  map[string]*totals
}

func (c *collector) run(ready, stopped *sync.WaitGroup) {
	ready.Done()
	defer stopped.Done()
	defer close(c.stopWorkers)
	for {
		select {
		case <-c.abort:
			return
		case sample := <-c.samples:
			c.sampleCount++
			t, ok := c.data[sample.operation]
			if !ok {
				panic(sample.operation)
			}
			t.update(sample.value)
		}
		if c.sampleCount >= c.maxSamples {
			logger.Debugf("%d samples, stopping", c.sampleCount)
			return
		}
		if c.sampleCount%500 == 0 {
			logger.Debugf("%d samples", c.sampleCount)
		}
	}
}

func (c *collector) results() string {
	var lines []string
	for name, t := range c.data {
		line := fmt.Sprintf("%-20s\t%s", name, t)
		lines = append(lines, line)
	}
	sort.Strings(lines)
	headers := "                    \t   count\t     min\t           max\t           avg\t        stddev"
	lines = append([]string{headers}, lines...)
	return strings.Join(lines, "\n")
}

// leaseStore implements lease.Store as simply as possible for use in
// the dummy provider. Heavily cribbed from raftlease.FSM.
type leaseStore struct {
	mu       sync.Mutex
	clock    clock.Clock
	entries  map[corelease.Key]*entry
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
		entries:  make(map[corelease.Key]*entry),
		target:   target,
		trapdoor: trapdoor,
	}
}

// Autoexpire is part of lease.Store.
func (*leaseStore) Autoexpire() bool { return false }

// ClaimLease is part of lease.Store.
func (s *leaseStore) ClaimLease(key corelease.Key, req corelease.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, found := s.entries[key]; found {
		return corelease.ErrInvalid
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
func (s *leaseStore) ExtendLease(key corelease.Key, req corelease.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, found := s.entries[key]
	if !found {
		return corelease.ErrInvalid
	}
	if entry.holder != req.Holder {
		return corelease.ErrInvalid
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
func (s *leaseStore) ExpireLease(key corelease.Key) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, found := s.entries[key]
	if !found {
		return corelease.ErrInvalid
	}
	expiry := entry.start.Add(entry.duration)
	if !s.clock.Now().After(expiry) {
		return corelease.ErrInvalid
	}
	delete(s.entries, key)
	s.target.Expired(key)
	return nil
}

// Leases is part of lease.Store.
func (s *leaseStore) Leases(keys ...corelease.Key) map[corelease.Key]corelease.Info {
	s.mu.Lock()
	defer s.mu.Unlock()

	filter := make(map[corelease.Key]bool)
	filtering := len(keys) > 0
	if filtering {
		for _, key := range keys {
			filter[key] = true
		}
	}

	results := make(map[corelease.Key]corelease.Info)
	for key, entry := range s.entries {
		if filtering && !filter[key] {
			continue
		}

		results[key] = corelease.Info{
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
func (s *leaseStore) PinLease(key corelease.Key, entity string) error {
	return errors.NotImplementedf("lease pinning")
}

// UnpinLease is part of lease.Store.
func (s *leaseStore) UnpinLease(key corelease.Key, entity string) error {
	return errors.NotImplementedf("lease unpinning")
}

// Pinned is part of the Store interface.
func (s *leaseStore) Pinned() map[corelease.Key][]string {
	return nil
}

type nullTarget struct{}

func (*nullTarget) Claimed(corelease.Key, string) {}
func (*nullTarget) Expired(corelease.Key)         {}

func nullTrapdoor(corelease.Key, string) corelease.Trapdoor {
	return nil
}

// nullSecretary implements lease.Secretary but doesn't do any checks.
type nullSecretary struct{}

func (nullSecretary) CheckLease(key corelease.Key) error         { return nil }
func (nullSecretary) CheckHolder(name string) error              { return nil }
func (nullSecretary) CheckDuration(duration time.Duration) error { return nil }
