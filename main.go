package main

import (
	"os"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
)

var (
	Version = "dev"

	GroupName = os.Getenv("GROUP_NAME")
)

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	cmd.RunWebhookServer(GroupName, &ExoscaleSolver{})
}
