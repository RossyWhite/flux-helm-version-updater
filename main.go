package main

import (
	"fmt"
	"github.com/RossyWhite/flux-helm-version-updater/cmd"
	"os"
)

func main() {
	if err := cmd.NewHelmVersionUpdateCmd().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
