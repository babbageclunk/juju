// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package featuretests

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	jujuos "github.com/juju/utils/os"
	"github.com/juju/utils/series"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/agent"
	"github.com/juju/juju/api"
	"github.com/juju/juju/apiserver/params"
	agentcmd "github.com/juju/juju/cmd/jujud/agent"
	"github.com/juju/juju/cmd/jujud/agent/agenttest"
	"github.com/juju/juju/controller"
	"github.com/juju/juju/core/auditlog"
	"github.com/juju/juju/state"
	"github.com/juju/juju/state/multiwatcher"
	coretesting "github.com/juju/juju/testing"
	"github.com/juju/juju/testing/factory"
	"github.com/juju/juju/worker/logsender"
)

var fastDialOpts = api.DialOpts{
	Timeout:    coretesting.LongWait,
	RetryDelay: coretesting.ShortWait,
}

type AuditLogSuite struct {
	agenttest.AgentSuite
	fakeEnsureMongo *agenttest.FakeEnsureMongo
	logger          *logsender.BufferedLogWriter
}

var _ = gc.Suite(&AuditLogSuite{})

func (s *AuditLogSuite) SetUpTest(c *gc.C) {
	if runtime.GOOS != "linux" {
		c.Skip(fmt.Sprintf("this test requires a controller, therefore does not support %q", runtime.GOOS))
	}
	currentSeries := series.MustHostSeries()
	osFromSeries, err := series.GetOSFromSeries(currentSeries)
	c.Assert(err, jc.ErrorIsNil)
	if osFromSeries != jujuos.Ubuntu {
		c.Skip(fmt.Sprintf("this test requires a controller, therefore does not support OS %q only Ubuntu", osFromSeries.String()))
	}

	// The default mongo port in controller config is 1234 - override
	// it with the real one from the running instance.  Without this
	// the model manifolds (including the log forwarder) never get
	// started since they depend on the state connection being up.
	mongod := testing.MgoServer
	host, portStr, err := net.SplitHostPort(mongod.Addr())
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(host, gc.Equals, "localhost")
	port, err := strconv.Atoi(portStr)
	c.Assert(err, jc.ErrorIsNil)

	s.ControllerConfigAttrs = map[string]interface{}{
		controller.AuditingEnabled: true,
		controller.StatePort:       port,
	}
	s.AgentSuite.SetUpTest(c)
	s.fakeEnsureMongo = agenttest.InstallFakeEnsureMongo(s)
	s.logger, err = logsender.InstallBufferedLogWriter(1000)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *AuditLogSuite) TestManageModelAuditsAPI(c *gc.C) {
	controllerConf, err := s.State.ControllerConfig()
	c.Assert(err, jc.ErrorIsNil)
	c.Logf("auditingEnabled: %v", controllerConf.AuditingEnabled())

	// Create a machine and an agent for it.
	m, password := s.Factory.MakeMachineReturningPassword(c, &factory.MachineParams{
		Nonce: agent.BootstrapNonce,
		Jobs:  []state.MachineJob{state.JobManageModel},
	})

	conf, _ := s.PrimeAgent(c, m.Tag(), password)
	agentConf := agentcmd.NewAgentConf(s.DataDir())
	err = agentConf.ReadConfig(m.Tag().String())
	c.Assert(err, jc.ErrorIsNil)

	machineAgentFactory := agentcmd.MachineAgentFactoryFn(
		agentConf,
		s.logger,
		agentcmd.DefaultIntrospectionSocketName,
		noPreUpgradeSteps,
		c.MkDir(),
	)
	a, err := machineAgentFactory(m.Id())
	c.Assert(err, jc.ErrorIsNil)

	// Ensure there's no logs to begin with.
	// Start the agent.
	go func() { c.Check(a.Run(nil), jc.ErrorIsNil) }()
	defer func() {
		c.Assert(a.Stop(), jc.ErrorIsNil)
	}()

	user := s.Factory.MakeUser(c, &factory.UserParams{
		Password: "blah",
	})

	logPath := filepath.Join(conf.LogDir(), "audit.log")

	apiInfo, ok := conf.APIInfo()
	c.Assert(ok, jc.IsTrue)
	apiInfo.Tag = user.Tag()
	apiInfo.Password = "blah"
	st, err := api.Open(apiInfo, fastDialOpts)
	c.Assert(err, jc.ErrorIsNil)
	defer st.Close()

	time.Sleep(30 * time.Second)
	client := st.Client()
	response, err := client.AddMachines([]params.AddMachineParams{{
		Jobs: []multiwatcher.MachineJob{"JobHostUnits"},
	}})
	c.Assert(err, jc.ErrorIsNil)
	c.Logf("**** response was %#v", response)

	// Check that there's a call to Client.AddMachinesV2 in the log.
	records := readAuditLog(c, logPath)
	c.Assert(records, gc.HasLen, 3)
	c.Assert(records[1].Request, gc.NotNil)
	c.Assert(records[1].Request.Facade, gc.Equals, "Client")
	c.Assert(records[1].Request.Method, gc.Equals, "AddMachinesV2")
}

func readAuditLog(c *gc.C, logPath string) []auditlog.Record {
	file, err := os.Open(logPath)
	c.Assert(err, jc.ErrorIsNil)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var results []auditlog.Record
	for scanner.Scan() {
		var record auditlog.Record
		err := json.Unmarshal(scanner.Bytes(), &record)
		c.Assert(err, jc.ErrorIsNil)
		results = append(results, record)
	}
	return results
}
