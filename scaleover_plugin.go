package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/andrew-d/go-termutil"
	"github.com/cloudfoundry/cli/plugin"
	"github.com/cloudfoundry/cli/cf/models"
	"github.com/guidowb/cf-go-client/panic"
	"github.com/guidowb/cf-go-client/api"
)

type ScaleoverCmd struct {
	app1 models.Application
	app2 models.Application
	cc   api.CloudController
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

	defer panic.HandlePanics()

	cmd.cc = api.NewCloudController()

	err := cmd.usage(args)
	checkError(err)

	rolloverTime, err := cmd.parseTime(args[3])
	checkError(err)

	cmd.app1, err = cmd.getAppStatus(args[1])
	checkError(err)

	cmd.app2, err = cmd.getAppStatus(args[2])
	checkError(err)

	count := cmd.app1.InstanceCount
	if (count == 0) {
		fmt.Println("There are no instances of the source app to scale over")
		os.Exit(0)
	}
	sleepInterval := time.Duration(rolloverTime.Nanoseconds() / int64(count))

	cmd.showStatus()

	for count > 0 {
		count--
		err = cmd.scaleUp(&cmd.app2)
		checkError(err)
		err = cmd.scaleDown(&cmd.app1)
		checkError(err)
		cmd.showStatus()
		if count > 0 {
			time.Sleep(sleepInterval)
		}
	}
	fmt.Println()
}

func (cmd *ScaleoverCmd) getAppStatus(name string) (app models.Application, err error) {
	app, err = cmd.cc.GetApplication(name)
	if nil != err {
		return
	}

	// Compensate for some CF weirdness that leaves the instance count non-zero
	// even though the app is stopped
	if "stopped" == app.State {
		app.InstanceCount = 0
	}
	return
}

func (cmd *ScaleoverCmd) scaleUp(app *models.Application) (err error) {
	// First set the desired instance count (even if the app is not already started)
	app.InstanceCount++
	err = cmd.cc.UpdateApplication(app, models.AppParams{InstanceCount: &app.InstanceCount})
	if nil != err {
		return
	}
	// If not already started, start it
	if app.State != "started" {
		err = cmd.cc.StartApplication(app)
	}
	return
}

func (cmd *ScaleoverCmd) scaleDown(app *models.Application) (err error) {
	app.InstanceCount--
	// If going to zero, stop the app
	if app.InstanceCount == 0 {
		err = cmd.cc.StopApplication(app)
		if err != nil {
			return
		}
		app.InstanceCount = 0
	} else {
		err = cmd.cc.UpdateApplication(app, models.AppParams{InstanceCount: &app.InstanceCount})
	}
	return
}

func (cmd *ScaleoverCmd) showStatus() {
	if termutil.Isatty(os.Stdout.Fd()) {
		fmt.Printf("%s (%s) %s %s %s (%s) \r",
			cmd.app1.Name,
			cmd.app1.State,
			strings.Repeat("<", cmd.app1.InstanceCount),
			strings.Repeat(">", cmd.app2.InstanceCount),
			cmd.app2.Name,
			cmd.app2.State,
		)
	} else {
		fmt.Printf("%s (%s) %d instances, %s (%s) %d instances\n",
			cmd.app1.Name,
			cmd.app1.State,
			cmd.app1.InstanceCount,
			cmd.app2.Name,
			cmd.app2.State,
			cmd.app2.InstanceCount,
		)
	}
}

func checkError(err error) {
	if nil != err {
		fmt.Println(err)
		os.Exit(1)
	}
}