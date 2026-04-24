package main

import (
	"os"

	"k8s.io/component-base/cli"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	"github.com/montypx/happy-edge-scheduling-plugin/pkg/plugins/happyedge"
)

func main() {
	command := app.NewSchedulerCommand(
		app.WithPlugin(happyedge.Name, happyedge.New),
	)

	code := cli.Run(command)
	os.Exit(code)
}
