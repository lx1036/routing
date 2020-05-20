package main

import (
	"k8s-lx1036/k8s-ui/backend/kubernetes/plugins/cicd/cmd"
	"os"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}