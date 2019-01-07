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
	"os"

	"github.com/spf13/cobra"
)

// trainCmd represents the train command
var genCompletions = &cobra.Command{
	Use:   "generate-completions",
	Aliases: []string {"genc"},
	Short: "Generate bash completions file",
	Run: func(cmd *cobra.Command, args []string) {
		// Make a bash completions file for the fun of it.  This will have to be copied to
		// /usr/local/etc/bash_completion.d/ffdl or someplace similar, and won't
		// go into effect until the next time you start a terminal
		// Experimental, commented out for now.  I guess there should be a command to do this.
		fileEx, _ := os.Executable()
		fmt.Printf("%s", fileEx)
		completionsFile := fileEx+"_completions.sh"
		rootCmd.GenBashCompletionFile(completionsFile)
		os.Chmod(completionsFile, 0x775 )
		fmt.Print("\n\n")
		fmt.Print("This might work to install the completions globally:\n")
		fmt.Printf("cp %s %s\n\n", completionsFile, "/usr/local/etc/bash_completion.d/ffdl")
		fmt.Print("Or execute in this terminal:\n")
		fmt.Printf(". %s\n", completionsFile)

		return
	},
}

func init() {
	rootCmd.AddCommand(genCompletions)
	//scaleTest.Flags().BoolVarP(&scaleTestVerbose, "verbose", "v", false,
	//	"if specified, be annoyingly verbose")
}
