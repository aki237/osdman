package call

import (
	"errors"
	"fmt"
	"net"
	"os"
	"osdman/pkg/config"
	"osdman/pkg/consts"

	"github.com/spf13/cobra"
)

const shellCompDirective = cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoFileComp

var Command = &cobra.Command{
	Use:   "call [domain] [verb]",
	Short: "RPC call for a domain and verb defined in the config",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, ok := cmd.Context().Value(consts.CtxVarConfig).(*config.Config)
		if !ok {
			return errors.New("configuration not passed in context")
		}

		runtimeSocket := fmt.Sprintf("/run/user/%d/osdcmd.sock", os.Getuid())

		conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{
			Name: runtimeSocket,
			Net:  "unixgram",
		})
		if err != nil {
			return err
		}
		defer conn.Close()

		n, err := conn.Write([]byte(fmt.Sprintf("%s/%s", args[0], args[1])))
		if err != nil {
			return err
		}

		fmt.Printf("Wrote: %d\n", n)

		return nil
	},

	Args: cobra.ExactValidArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 2 {
			return []string{}, shellCompDirective
		}

		cfg, ok := cmd.Context().Value(consts.CtxVarConfig).(*config.Config)
		if !ok {
			return []string{}, shellCompDirective
		}

		switch len(args) {
		case 0:
			domains := make([]string, 0, len(cfg.Domains))
			for k := range cfg.Domains {
				domains = append(domains, k)
			}

			return domains, shellCompDirective
		case 1:
			domain, ok := cfg.Domains[args[0]]
			if !ok {
				return []string{}, cobra.ShellCompDirectiveError | shellCompDirective
			}

			verbs := make([]string, 0, len(domain.Verbs))
			for k := range domain.Verbs {
				verbs = append(verbs, k)
			}

			return verbs, shellCompDirective
		default:
			return []string{}, shellCompDirective
		}
	},
}
