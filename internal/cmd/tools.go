package cmd

import "github.com/spf13/cobra"

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Inspect registered tools",
}

var toolsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available tools (capability registry)",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		caps := app.AgentCoordinator.Capabilities()
		if len(caps) == 0 {
			cmd.Println("No tools registered.")
			return nil
		}
		for _, c := range caps {
			cmd.Printf("- %s: %s\n", c.ID, c.Description)
		}
		return nil
	},
}

func init() {
	toolsCmd.AddCommand(toolsListCmd)
	rootCmd.AddCommand(toolsCmd)
}
