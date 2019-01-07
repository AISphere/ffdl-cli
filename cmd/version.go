// Copyright © 2019 IBM
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var follow, metrics, jsonformat bool

func init() {
	// versionCmd.PersistentFlags().BoolVarP(&follow, "follow", "f", false, "If specified, follow the log")
	// viper.BindPFlag("follow", versionCmd.PersistentFlags().Lookup("follow"))
	// versionCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.AddCommand(versionCmd)
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show CLI version and build time",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("CLI version: FfDL_CLI 1.0")
		fmt.Println("Build Time : 20190101")
	},
}
