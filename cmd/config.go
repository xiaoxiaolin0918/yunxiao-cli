package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Global CLI configuration management",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show resolved configuration (token masked)",
	Long:  "Risk: read",
	Run: func(cmd *cobra.Command, args []string) {
		r, err := config.Resolve()
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(output.Success(map[string]any{
			"api_base_url":    r.APIBaseURL,
			"organization_id": r.OrganizationID,
			"edition":         r.Edition,
			"token_source":    r.TokenSource,
			"token_masked":    config.MaskToken(r.AccessToken),
			"config_path":     r.ConfigPath,
		}, nil))
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config key (api_base_url|organization_id|edition|access_token|profile)",
	Long:  "Risk: write",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		f, _, err := config.LoadFile()
		if err != nil {
			handleErr(err)
			return
		}
		key, val := args[0], args[1]
		switch strings.ToLower(key) {
		case "api_base_url", "api-base-url":
			f.APIBaseURL = strings.TrimRight(val, "/")
		case "organization_id", "organization-id":
			f.OrganizationID = val
		case "edition":
			f.Edition = val
		case "access_token", "access-token", "token":
			f.AccessToken = val
		case "profile":
			f.Profile = val // default tenant profile (#130); empty clears it
		default:
			handleErr(fmt.Errorf("unknown key %q", key))
			return
		}
		p, err := config.SaveFile(f)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(output.Success(map[string]any{"config_path": p, "key": key}, nil))
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print config file path",
	Long:  "Risk: read",
	Run: func(cmd *cobra.Command, args []string) {
		p, err := config.Path()
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(output.Success(map[string]any{"path": p}, nil))
	},
}

func init() {
	configCmd.AddCommand(configShowCmd, configSetCmd, configPathCmd)
}
