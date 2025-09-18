package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "kubectl-tenant",
	Short: "CLI for managing Tenant CRs from Stakater Tenant Operator",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
