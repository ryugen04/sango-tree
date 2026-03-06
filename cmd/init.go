package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// sango init で生成するテンプレート
const sangoYAMLTemplate = `name: my-project
version: "1.0"

services:
  # example:
  #   type: process
  #   port: 3000
  #   command: npm start

ports:
  strategy: fixed
  range: [3000, 9999]

doctor:
  checks:
    - name: Git
      command: git --version
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "sango.yaml のテンプレートを生成する",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "sango.yaml"

		// 既にファイルが存在する場合はエラー
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("sango.yaml は既に存在します")
		}

		if err := os.WriteFile(path, []byte(sangoYAMLTemplate), 0644); err != nil {
			return fmt.Errorf("sango.yaml の作成に失敗: %w", err)
		}

		fmt.Println("sango.yaml を作成しました")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
