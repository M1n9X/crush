package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/spf13/cobra"
)

var permissionsCmd = &cobra.Command{
	Use:   "permissions",
	Short: "Manage stored tool permissions",
}

var permissionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List persistent tool permissions for this workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		cfg, err := config.Init(cwd, "", false)
		if err != nil {
			return err
		}
		path := permission.PersistentPath(cfg.Options.DataDirectory, cfg.WorkingDir())
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				cmd.Println("No persistent permissions stored.")
				return nil
			}
			return err
		}
		cmd.Printf("Workspace: %s\nStore: %s\n\n", cfg.WorkingDir(), path)
		cmd.Println(string(data))
		return nil
	},
}

var permissionsGrantCmd = &cobra.Command{
	Use:   "grant",
	Short: "Persistently allow a tool/action for this workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		tool, _ := cmd.Flags().GetString("tool")
		action, _ := cmd.Flags().GetString("action")
		pathFlag, _ := cmd.Flags().GetString("path")
		session, _ := cmd.Flags().GetString("session")
		if tool == "" || action == "" {
			return fmt.Errorf("tool and action are required")
		}
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		cfg, err := config.Init(cwd, "", false)
		if err != nil {
			return err
		}
		storePath := permission.PersistentPath(cfg.Options.DataDirectory, cfg.WorkingDir())
		existing, _ := permission.LoadPersistent(storePath)
		entry := permission.PermissionRequest{
			ToolName:  tool,
			Action:    action,
			Path:      pathFlag,
			SessionID: session,
		}
		if entry.SessionID == "" {
			entry.SessionID = "*"
		}
		existing = append(existing, entry)
		if err := permission.SavePersistent(storePath, existing); err != nil {
			return err
		}
		cmd.Printf("Granted %s:%s at %s (session=%s)\n", tool, action, storePath, entry.SessionID)
		return nil
	},
}

var permissionsRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Remove a persistent permission for this workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		tool, _ := cmd.Flags().GetString("tool")
		action, _ := cmd.Flags().GetString("action")
		pathFlag, _ := cmd.Flags().GetString("path")
		session, _ := cmd.Flags().GetString("session")
		if tool == "" || action == "" {
			return fmt.Errorf("tool and action are required")
		}
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		cfg, err := config.Init(cwd, "", false)
		if err != nil {
			return err
		}
		storePath := permission.PersistentPath(cfg.Options.DataDirectory, cfg.WorkingDir())
		existing, err := permission.LoadPersistent(storePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		filtered := existing[:0]
		for _, p := range existing {
			if p.ToolName == tool && p.Action == action {
				if session != "" && session != "*" && p.SessionID != session {
					filtered = append(filtered, p)
					continue
				}
				if pathFlag != "" && p.Path != pathFlag {
					filtered = append(filtered, p)
					continue
				}
				continue
			}
			filtered = append(filtered, p)
		}
		if err := permission.SavePersistent(storePath, filtered); err != nil {
			return err
		}
		cmd.Printf("Revoked %s:%s\n", tool, action)
		return nil
	},
}

var permissionsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear persistent permissions for this workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		cfg, err := config.Init(cwd, "", false)
		if err != nil {
			return err
		}
		path := permission.PersistentPath(cfg.Options.DataDirectory, cfg.WorkingDir())
		if path == "" {
			return fmt.Errorf("unable to resolve permission store path")
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		dir := filepath.Dir(path)
		entries, _ := os.ReadDir(dir)
		if len(entries) == 0 {
			_ = os.Remove(dir)
		}
		cmd.Printf("Cleared persistent permissions at %s\n", path)
		return nil
	},
}

var permissionsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export persistent permissions as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		cfg, err := config.Init(cwd, "", false)
		if err != nil {
			return err
		}
		path := permission.PersistentPath(cfg.Options.DataDirectory, cfg.WorkingDir())
		perms, err := permission.LoadPersistent(path)
		if err != nil {
			return err
		}
		enc, err := json.MarshalIndent(perms, "", "  ")
		if err != nil {
			return err
		}
		cmd.Println(string(enc))
		return nil
	},
}

func init() {
	permissionsGrantCmd.Flags().String("tool", "", "Tool name")
	permissionsGrantCmd.Flags().String("action", "", "Action name")
	permissionsGrantCmd.Flags().String("path", "", "Path to scope the grant (optional)")
	permissionsGrantCmd.Flags().String("session", "", "Session ID to scope the grant (default: *)")
	permissionsRevokeCmd.Flags().String("tool", "", "Tool name")
	permissionsRevokeCmd.Flags().String("action", "", "Action name")
	permissionsRevokeCmd.Flags().String("path", "", "Path to match (optional)")
	permissionsRevokeCmd.Flags().String("session", "", "Session ID to match (default: any)")

	permissionsCmd.AddCommand(
		permissionsListCmd,
		permissionsGrantCmd,
		permissionsRevokeCmd,
		permissionsClearCmd,
		permissionsExportCmd,
	)
	rootCmd.AddCommand(permissionsCmd)
}
