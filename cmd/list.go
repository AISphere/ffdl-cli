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
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/AISphere/ffdl-trainer/trainer/grpc_trainer_v2"

	"github.com/cenkalti/backoff"
	"github.com/spf13/cobra"
)

var listShort = false

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all of models",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			fmt.Println("Incorrect usage\ndlaas list -h for more details")
			return
		}
		var userID = os.Getenv("DLAAS_USERID")
		if userID == "" {
			fmt.Println("`DLAAS_USERID` is not set")
			return
		}
		if initTrainerClient() {
			//fmt.Println("Getting all training Jobs...")
			var resp *grpc_trainer_v2.GetAllResponse
			err := backoff.Retry(func() error { // retry with backoff
				var err error
				d := time.Now().Add(1 * time.Minute)
				ctx, cancel := context.WithDeadline(context.Background(), d)
				defer cancel()
				resp, err = dlaasGrpcClient.Client().GetAllTrainingsJobs(ctx, &grpc_trainer_v2.GetAllRequest{
					UserId: userID,
				})

				// if trainer returns error, start to retry
				if err != nil {
					fmt.Println("dlaasGrpcClient.Client().GetAllTrainingsJobs(...) failed.")
				}

				// if no err, quit backoff
				return nil
			}, defaultBackoff)
			defaultBackoff.Reset()

			if err != nil {
				fmt.Println(err.Error())
			} else if resp != nil {

				if listShort {
					for i := 0; i < len(resp.Jobs); i++ {
						job := resp.Jobs[i]
						fmt.Printf("%s\n", job.TrainingId)
					}
				} else {
					w := new(tabwriter.Writer)
					w.Init(os.Stdout, 10, 1, 1, ' ', 0)
					fmt.Fprintln(w, "ID\tNAME\tFRAMEWORKS\tSTATUS\tSTART\tCOMPLETED\t")
					for i := 0; i < len(resp.Jobs); i++ {
						job := resp.Jobs[i]
						fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", job.TrainingId, job.ModelDefinition.Name,
							job.ModelDefinition.Framework.Name, job.Status.StatusMessage,
							timeConver(job.Status.SubmissionTimestamp), timeConver(job.Status.CompletionTimestamp))
					}
					w.Flush()
				}
			} else {
				fmt.Println("No response from trainer")
			}
		}
	},
}

func timeConver(timestamp string) string {
	if _, err := strconv.Atoi(timestamp); err != nil {
		return timestamp
	}
	i, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		fmt.Println(err.Error())
		return timestamp
		//TODO: Actually I should grep error 'strconv.ParseInt: parsing'. Talked with Atin and will spend more timehere.
	}
	tm := time.Unix(i/1000, 0).String()
	return tm
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVarP(&listShort, "short", "s", false,
		"if specified, just print the training id")

}
