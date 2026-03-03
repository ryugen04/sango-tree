package cmd

import (
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "サービスを停止する",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		return runDown(cfg, args)
	},
}

func init() {
	downCmd.Flags().BoolVar(&downAll, "all", false, "sharedサービスも含めて全て停止する")
	rootCmd.AddCommand(downCmd)
}
