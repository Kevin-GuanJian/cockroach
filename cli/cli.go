package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/cockroachdb/cockroach/util"
)

var osExit = os.Exit

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "output version information",
	Long:  `Output build version information.`,
	Run: func(cmd *cobra.Command, args []string) {
		info := util.GetBuildInfo()
		tw := tabwriter.NewWriter(os.Stdout, 2, 1, 2, ' ', 0)
		fmt.Fprintf(tw, "Build Vers:  %s\n", info.Vers)
		fmt.Fprintf(tw, "Build Tag:   %s\n", info.Tag)
		fmt.Fprintf(tw, "Build Time:  %s\n", info.Time)
		fmt.Fprintf(tw, "Build Deps:\n\t%s\n",
			strings.Replace(strings.Replace(info.Deps, " ", "\n\t", -1), ":", "\t", -1))
		_ = tw.Flush()
	},
}

var dmatrixCmd = &cobra.Command{
	Use: "dmatrix",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		context.Addr = util.EnsureHostPort(context.Addr)
	},
}

func init() {
	dmatrixCmd.AddCommand(
		startCmd,
		exterminateCmd,
		quitCmd,
		versionCmd,
	)
}

// Run ...
func Run(args []string) error {
	dmatrixCmd.SetArgs(args)
	return dmatrixCmd.Execute()
}

func mustUsage(cmd *cobra.Command) {
	if err := cmd.Usage(); err != nil {
		panic(err)
	}
}
