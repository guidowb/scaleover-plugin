package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/andrew-d/go-termutil"
	"github.com/cloudfoundry/cli/plugin"
)

type AppStatus struct {
	name           string
	countRunning   int
	countRequested int
	state          string
}

type ScaleoverCmd struct {
	app1 AppStatus
	app2 AppStatus
	cc   CloudController
}

//GetMetadata returns metatada
func (cmd *ScaleoverCmd) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "scaleover",
		Version: plugin.VersionType{
			Major: 0,
			Minor: 2,
			Build: 0,
		},
		Commands: []plugin.Command{
			{
				Name:     "scaleover",
				HelpText: "Roll http traffic from one application to another",
				UsageDetails: plugin.Usage{
					Usage: "cf scaleover APP1 APP2 ROLLOVER_DURATION",
				},
			},
		},
	}
}

func main() {
	plugin.Start(new(ScaleoverCmd))
}

func (cmd *ScaleoverCmd) usage(args []string) error {
	if 4 != len(args) {
		return errors.New("Usage: cf scaleover\n\tcf scaleover APP1 APP2 ROLLOVER_DURATION")
	}
	return nil
}

func (cmd *ScaleoverCmd) parseTime(duration string) (time.Duration, error) {
	rolloverTime := time.Duration(0)
	var err error
	rolloverTime, err = time.ParseDuration(duration)
	if err != nil {
		return rolloverTime, err
	}
	if 0 > rolloverTime {
		return rolloverTime, errors.New("Duration must be a positive number in the format of 1m")
	}

	return rolloverTime, nil
}

func (cmd *ScaleoverCmd) Run(cliConnection plugin.CliConnection, args []string) {

	if args[0] == "scaleover" {
		cmd.ScaleoverCommand(cliConnection, args)
	}
}

func (cmd *ScaleoverCmd) ScaleoverCommand(cliConnection plugin.CliConnection, args []string) {

	defer handlePanics()

	cmd.cc = NewCloudController()

	err := cmd.usage(args)
	if nil != err {
		fmt.Println(err)
		os.Exit(1)
	}

	rolloverTime, err := cmd.parseTime(args[3])
	if nil != err {
		fmt.Println(err)
		os.Exit(1)
	}

	// The getAppStatus calls will exit with an error if the named apps don't exist
	cmd.app1, err = cmd.getAppStatus(cliConnection, args[1])
	if nil != err {
		fmt.Println(err)
		os.Exit(1)
	}

	cmd.app2, err = cmd.getAppStatus(cliConnection, args[2])
	if nil != err {
		fmt.Println(err)
		os.Exit(1)
	}

	cmd.showStatus()

	count := cmd.app1.countRequested
	sleepInterval := time.Duration(rolloverTime.Nanoseconds() / int64(count))

	for count > 0 {
		count--
		cmd.app2.scaleUp(cliConnection)
		cmd.app1.scaleDown(cliConnection)
		cmd.showStatus()
		if count > 0 {
			time.Sleep(sleepInterval)
		}
	}
	fmt.Println()
}

func (cmd *ScaleoverCmd) getAppStatus(cliConnection plugin.CliConnection, name string) (AppStatus, error) {
	status := AppStatus{
		name:           name,
		countRunning:   0,
		countRequested: 0,
		state:          "unknown",
	}

	app, err := cmd.cc.GetApplication(name)
	if (err != nil) {
		return status, err
	}

	status.state = app.State
	status.countRunning = app.RunningInstances
	status.countRequested = app.InstanceCount

	// Compensate for some CF weirdness that leaves the requested instances non-zero
	// even though the app is stopped
	if "stopped" == status.state {
		status.countRequested = 0
	}
	return status, nil
}

func (app *AppStatus) scaleUp(cliConnection plugin.CliConnection) {
	// If not already started, start it
	if app.state != "started" {
		cliConnection.CliCommandWithoutTerminalOutput("start", app.name)
		app.state = "started"
	}
	app.countRequested++
	cliConnection.CliCommandWithoutTerminalOutput("scale", "-i", strconv.Itoa(app.countRequested), app.name)
}

func (app *AppStatus) scaleDown(cliConnection plugin.CliConnection) {
	app.countRequested--
	// If going to zero, stop the app
	if app.countRequested == 0 {
		cliConnection.CliCommandWithoutTerminalOutput("stop", app.name)
		app.state = "stopped"
	} else {
		cliConnection.CliCommandWithoutTerminalOutput("scale", "-i", strconv.Itoa(app.countRequested), app.name)
	}
}

func (cmd *ScaleoverCmd) showStatus() {
	if termutil.Isatty(os.Stdout.Fd()) {
		fmt.Printf("%s (%s) %s %s %s (%s) \r",
			cmd.app1.name,
			cmd.app1.state,
			strings.Repeat("<", cmd.app1.countRequested),
			strings.Repeat(">", cmd.app2.countRequested),
			cmd.app2.name,
			cmd.app2.state,
		)
	} else {
		fmt.Printf("%s (%s) %d instances, %s (%s) %d instances\n",
			cmd.app1.name,
			cmd.app1.state,
			cmd.app1.countRequested,
			cmd.app2.name,
			cmd.app2.state,
			cmd.app2.countRequested,
		)
	}
}
