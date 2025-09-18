package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stakater/kubectl-tenant/internal/config"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage kubectl-tenant feature flags and settings",
}

var listConfigCmd = &cobra.Command{
	Use:   "list",
	Short: "List current feature flag status",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger, err := zap.NewProduction()
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer logger.Sync()

		ff, err := config.LoadOrCreateConfig()
		if err != nil {
			return err
		}

		fmt.Println("Feature Flags:")
		fmt.Println("==============")

		// Get all features and sort for consistent output
		var features []featureflags.Feature
		for feature := range ff.Flags {
			features = append(features, feature)
		}
		sort.Slice(features, func(i, j int) bool {
			return string(features[i]) < string(features[j])
		})

		for _, feature := range features {
			flag := ff.Flags[feature]
			status := "✅ Enabled"
			if !flag.Enabled {
				status = "❌ Disabled"
			}
			source := ""
			if flag.Source != "" {
				source = fmt.Sprintf(" (source: %s)", flag.Source)
			}
			fmt.Printf("%-25s %s%s\n", feature, status, source)
		}

		fmt.Printf("\nConfig file: %s\n", config.GetConfigPath())
		return nil
	},
}

var enableCmd = &cobra.Command{
	Use:   "enable <feature>",
	Short: "Enable a feature flag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		featureName := args[0]
		logger, err := zap.NewProduction()
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer logger.Sync()

		ff, err := config.LoadOrCreateConfig()
		if err != nil {
			return err
		}

		feature := featureflags.Feature(strings.ToLower(featureName))
		ff.Enable(feature)

		if err := config.SaveConfig(ff); err != nil {
			return err
		}

		fmt.Printf("✅ Feature '%s' has been enabled.\n", feature)
		fmt.Printf("Config file: %s\n", config.GetConfigPath())
		return nil
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable <feature>",
	Short: "Disable a feature flag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		featureName := args[0]
		logger, err := zap.NewProduction()
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer logger.Sync()

		ff, err := config.LoadOrCreateConfig()
		if err != nil {
			return err
		}

		feature := featureflags.Feature(strings.ToLower(featureName))
		ff.Disable(feature)

		if err := config.SaveConfig(ff); err != nil {
			return err
		}

		fmt.Printf("✅ Feature '%s' has been disabled.\n", feature)
		fmt.Printf("Config file: %s\n", config.GetConfigPath())
		return nil
	},
}

func init() {
	configCmd.AddCommand(listConfigCmd)
	configCmd.AddCommand(enableCmd)
	configCmd.AddCommand(disableCmd)
	rootCmd.AddCommand(configCmd)
}
