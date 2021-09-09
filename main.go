package main

import (
	"fmt"
	"os"

	"github.com/RossyWhite/flux-helm-version-updater/cmd"
)

func main() {
	if err := cmd.NewHelmVersionUpdateCmd().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
