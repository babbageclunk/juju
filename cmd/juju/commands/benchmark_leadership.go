// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package commands

import (
	"io/ioutil"
	"math"
	"strings"
	"time"

	"github.com/juju/cmd"
	"github.com/juju/errors"
	"github.com/juju/gnuflag"
	"github.com/juju/naturalsort"
	"github.com/juju/utils"
	"gopkg.in/juju/names.v2"
	"gopkg.in/juju/worker.v1"
	"gopkg.in/tomb.v2"
	"gopkg.in/yaml.v2"

	"github.com/juju/juju/api"
	leadershipapi "github.com/juju/juju/api/leadership"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/core/leadership"
	"github.com/juju/juju/worker/catacomb"
)

var benchmarkSummary = `
Makes lots of leadership claims and reports on timings.`[1:]

var benchmarkDetails = `

Takes a yaml file with connection details for a number of units so
that it can pose as the unit agents to make leadership claims. Makes
leadership claims as fast as possible for those units (for leadership
of the applications), collecting time taken, and reports.

The yaml file passed in should look like:

mysql/0: <password>
prometheus/1: <password>


Examples:
    juju benchmark-leadership 60 units.yaml
`[1:]

// NewBenchmarkCommand returns a command to benchmark leadership
// claims.
func NewBenchmarkCommand() modelcmd.ModelCommand {
	return modelcmd.Wrap(&benchmarkCommand{})
}

// benchmarkCommand is responsible for benchmarking leadership
// claims.
type benchmarkCommand struct {
	modelcmd.ModelCommandBase
	catacomb catacomb.Catacomb
	units    map[string]string
	runtime  int
	factor   int
}

func (c *benchmarkCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "benchmark-leadership",
		Args:    "units-yaml",
		Purpose: benchmarkSummary,
		Doc:     benchmarkDetails,
	}
}

func (c *benchmarkCommand) SetFlags(f *gnuflag.FlagSet) {
	f.IntVar(&c.runtime, "t", 60, "Number of seconds to run the benchmark")
	f.IntVar(&c.runtime, "time", 60, "")
	f.IntVar(&c.factor, "f", 1, "Number of goroutines to run per unit")
	f.IntVar(&c.factor, "factor", 1, "")
}

func (c *benchmarkCommand) Init(args []string) error {
	if len(args) == 0 {
		return errors.New("no unit yaml specified")
	}
	if err := c.readUnits(args[0]); err != nil {
		return errors.Trace(err)
	}
	return cmd.CheckEmpty(args[1:])
}

func (c *benchmarkCommand) readUnits(filename string) error {
	path, err := utils.NormalizePath(filename)
	if err != nil {
		return errors.Trace(err)
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Trace(err)
	}
	if err := yaml.Unmarshal(data, &c.units); err != nil {
		return errors.Trace(err)
	}
	return nil
}

type closableClaimer struct {
	leadership.Claimer
	close func() error
}

func (c *benchmarkCommand) getAPI(unit string) (*closableClaimer, error) {
	info, err := c.connectionInfo(unit)
	if err != nil {
		return nil, errors.Trace(err)
	}
	root, err := api.Open(info, api.DefaultDialOpts())
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &closableClaimer{
		Claimer: leadershipapi.NewClient(root),
		close:   root.Close,
	}, nil
}

func (c *benchmarkCommand) connectionInfo(unit string) (*api.Info, error) {
	controllerName, err := c.ControllerName()
	if err != nil {
		return nil, errors.Trace(err)
	}
	store := c.ClientStore()
	controller, err := store.ControllerByName(controllerName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	modelName, err := c.ModelName()
	if err != nil {
		return nil, errors.Trace(err)
	}
	model, err := store.ModelByName(controllerName, modelName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &api.Info{
		Addrs:    controller.APIEndpoints,
		CACert:   controller.CACert,
		ModelTag: names.NewModelTag(model.ModelUUID),
		Tag:      names.NewUnitTag(unit),
		Password: c.units[unit],
	}, nil
}

func (c *benchmarkCommand) Run(ctx *cmd.Context) error {
	var names []string
	for u := range c.units {
		names = append(names, u)
	}
	names = naturalsort.Sort(names)
	ctx.Infof("running for %ds against: %s", c.runtime, strings.Join(names, ","))
	samples := make(chan time.Duration)
	collector := newCollector(samples)
	err := catacomb.Invoke(catacomb.Plan{
		Site: &c.catacomb,
		Work: func() error {
			return c.runUnitWorkers(samples)
		},
	})
	if err != nil {
		collector.Kill()
		return errors.Trace(err)
	}

	select {
	case <-c.catacomb.Dying():
		worker.Stop(collector)
		return errors.Trace(c.catacomb.Wait())
	case <-time.After(time.Duration(c.runtime) * time.Second):
		c.catacomb.Kill(nil)
		err := c.catacomb.Wait()
		close(samples)
		collector.Wait()
		if err != nil {
			return errors.Trace(err)
		}
	}
	ctx.Infof("total requests = %d", collector.count)
	ctx.Infof("total duration = %.2f", collector.total.Seconds())
	ctx.Infof("requests/sec = %.2f", float64(collector.count)/collector.total.Seconds())
	ctx.Infof("mean = %.0fms, stddev = %.0fms", collector.m*1000, collector.stddev()*1000)
	ctx.Infof("fastest: %.0fms, slowest %.0fms", collector.min.Seconds()*1000.0, collector.max.Seconds()*1000.0)
	return nil
}

func (c *benchmarkCommand) runUnitWorkers(samples chan<- time.Duration) error {
	for i := 0; i < c.factor; i++ {
		for unit := range c.units {
			unit := unit
			w, err := newUnitWorker(unit, samples, func() (*closableClaimer, error) {
				return c.getAPI(unit)
			})
			if err != nil {
				return errors.Trace(err)
			}
			err = c.catacomb.Add(w)
			if err != nil {
				return errors.Trace(err)
			}
		}
	}
	// Wait until we get killed or one of the workers hits an error.
	<-c.catacomb.Dying()
	return nil
}

func newUnitWorker(unit string, target chan<- time.Duration, getClaimer func() (*closableClaimer, error)) (*unitWorker, error) {
	application, err := names.UnitApplication(unit)
	if err != nil {
		return nil, errors.Trace(err)
	}
	w := unitWorker{
		application: application,
		unit:        unit,
		getClaimer:  getClaimer,
		target:      target,
	}
	w.tomb.Go(w.loop)
	return &w, nil
}

type unitWorker struct {
	tomb        tomb.Tomb
	application string
	unit        string
	getClaimer  func() (*closableClaimer, error)
	target      chan<- time.Duration
}

func (w *unitWorker) Kill() {
	w.tomb.Kill(nil)
}

func (w *unitWorker) Wait() error {
	return w.tomb.Wait()
}

func (w *unitWorker) loop() error {
	logger.Debugf("starting worker for %s", w.unit)
	claimer, err := w.getClaimer()
	if err != nil {
		return errors.Trace(err)
	}
	for {
		select {
		case <-w.tomb.Dying():
			logger.Debugf("stopping worker for %s", w.unit)
			return claimer.close()
		default:
		}
		start := time.Now()
		err := claimer.ClaimLeadership(w.application, w.unit, 30*time.Second)
		if err != nil {
			return errors.Trace(err)
		}
		end := time.Now()
		w.target <- end.Sub(start)
	}
}

func newCollector(samples <-chan time.Duration) *collector {
	c := collector{samples: samples}
	c.tomb.Go(c.loop)
	return &c
}

type collector struct {
	tomb            tomb.Tomb
	samples         <-chan time.Duration
	count           int
	total, max, min time.Duration
	m, s            float64
}

func (c *collector) Kill() {
	c.tomb.Kill(nil)
}

func (c *collector) Wait() error {
	return c.tomb.Wait()
}

func (c *collector) loop() error {
	logger.Debugf("collector starting")
	defer logger.Debugf("collector stopped")
	// Standard deviation using Welford's method
	// https://www.johndcook.com/blog/standard_deviation/
	var oldM, oldS float64
	for {
		select {
		case <-c.tomb.Dying():
			return nil
		case sample, ok := <-c.samples:
			if !ok {
				return nil
			}
			c.count++
			c.total += sample
			if sample < c.min || c.min == 0 {
				c.min = sample
			}
			if sample > c.max {
				c.max = sample
			}

			seconds := sample.Seconds()
			if c.count == 1 {
				oldM = seconds
				c.m = seconds
				oldS = 0
			} else {
				c.m = oldM + (seconds-oldM)/float64(c.count)
				c.s = oldS + (seconds-oldM)*(seconds-c.m)

				oldM = c.m
				oldS = c.s
			}
		}
	}
}

func (c *collector) stddev() float64 {
	if c.count > 1 {
		return math.Sqrt(c.s / float64(c.count-1))
	}
	return 0
}
