package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const version = "0.1.21-gds"

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "amgctl",
		Short: "AMG Control CLI - GDS Configuration Tool",
		Long: `amgctl is a command line interface for managing GPU Direct Storage (GDS) configuration.
This minimal version focuses on GDS setup, cufile.json configuration, and validation.`,
		Version:    version,
		SuggestFor: []string{"status"}, // Suggest "amgctl" when user types "amgctl status"
	}
)

func Execute() {
	// Configure Cobra to provide better error handling
	rootCmd.SilenceErrors = true           // Prevent duplicate error messages
	rootCmd.SuggestionsMinimumDistance = 1 // More sensitive to typos

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		// Provide specific guidance for common mistakes
		if err.Error() == `unknown command "pre-flight" for "amgctl"` {
			fmt.Fprintf(os.Stderr, "\nDid you mean:\n  amgctl host pre-flight\n")
		}

		fmt.Fprintf(os.Stderr, "\nRun 'amgctl --help' for usage.\n")
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/amgctl.yaml)")
	// Add subcommands (only host for GDS)
	rootCmd.AddCommand(hostCmd)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".config/amgctl" (without extension).
		viper.AddConfigPath(fmt.Sprintf("%s/.config", home))
		viper.SetConfigName("amgctl")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("AMGCTL")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
