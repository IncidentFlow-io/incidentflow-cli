package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "incidentflow",
	Short: "IncidentFlow CLI — manage clusters and agents",
	Long: `incidentflow is the official CLI for IncidentFlow.

Install and manage IncidentFlow Agents in Kubernetes clusters,
monitor cluster status, and interact with the IncidentFlow platform.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(whoamiCmd)
	rootCmd.AddCommand(clusterCmd)
}
