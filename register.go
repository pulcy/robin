package main

import (
	"github.com/spf13/cobra"
)

var (
	cmdRegister = &cobra.Command{
		Use:   "register",
		Short: "Perform offline registration",
		Long:  "Perform offline registration",
		Run:   UsageFunc,
	}
)

func init() {
	cmdMain.AddCommand(cmdRegister)
}
