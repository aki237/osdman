package main

import (
	"context"
	"log"
	"os"
	"osdman/cmd/call"
	"osdman/cmd/daemon"
	"osdman/pkg/config"
	"osdman/pkg/consts"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var rootCmd = &cobra.Command{
	Use:   "osdman",
	Short: "osdman is used to run OSDs for non traditional wayland desktop setups using wob",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return err
		}

		configFile := filepath.Join(configDir, "osdman", "config.yaml")
		data, err := os.ReadFile(configFile)
		if err != nil {
			return err
		}

		var cfg config.Config

		err = yaml.Unmarshal(data, &cfg)
		if err != nil {
			return err
		}

		cmd.SetContext(context.WithValue(cmd.Context(), consts.CtxVarConfig, &cfg))

		return nil
	},
}

func main() {
	rootCmd.AddCommand(daemon.Command)
	rootCmd.AddCommand(call.Command)

	err := rootCmd.Execute()
	if err != nil {
		log.Fatalf("error running osdcmd: %s", err)
	}
}
