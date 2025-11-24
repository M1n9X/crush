package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "Inspect plugins",
}

var pluginsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()
		all := app.Plugins.List()
		if len(all) == 0 {
			cmd.Println("No plugins registered.")
			return nil
		}
		for _, p := range all {
			policy := p.Sandbox
			if policy == "" {
				policy = "default"
			}
			cmd.Printf("- %s (api=%s enabled=%t sandbox=%s path=%s)\n", p.Name, p.APIVersion, p.Enabled, policy, p.Path)
		}
		return nil
	},
}

var pluginsDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable a plugin for this workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()
		app.Plugins.Disable(name)
		cmd.Printf("Disabled plugin %s (state stored per workspace)\n", name)
		return nil
	},
}

var pluginsEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable a plugin for this workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()
		app.Plugins.Enable(name)
		cmd.Printf("Enabled plugin %s\n", name)
		return nil
	},
}

func init() {
	pluginsDisableCmd.Flags().String("name", "", "Plugin name to disable")
	pluginsEnableCmd.Flags().String("name", "", "Plugin name to enable")

	pluginsCmd.AddCommand(pluginsListCmd)
	pluginsCmd.AddCommand(pluginsDisableCmd)
	pluginsCmd.AddCommand(pluginsEnableCmd)
	rootCmd.AddCommand(pluginsCmd)
}
