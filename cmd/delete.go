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
	"os/exec"
	"time"

	"github.com/AISphere/ffdl-trainer/trainer/grpc_trainer_v2"

	"github.com/cenkalti/backoff"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var deleteAll = false

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete TRAINING_ID",
	Short: "Delete a model",
	Run: func(cmd *cobra.Command, args []string) {

		if deleteAll {
			command := "dlaas list -s | xargs -n1 dlaas delete"
			output, error := exec.Command("bash","-c", command).Output()
			if error != nil {
				fmt.Sprintf("Error during execution of command %s: %s", command, error)
			} else {
				fmt.Sprint(output)
			}
			return
		}

		if len(args) != 1 {
			fmt.Println("Incorrect usage\ndlaas delete -h for more details")
			return
		}
		var userID = os.Getenv("DLAAS_USERID")
		if userID == "" {
			fmt.Println("`DLAAS_USERID` is not set")
			return
		}
		if initTrainerClient() {
			var trainingID = args[0]
			if !IsValidTrainingID(trainingID, userID, true) {
				return
			}

			fmt.Printf("Deleting model '%s' ...\n", trainingID)
			var resp *grpc_trainer_v2.DeleteResponse
			err := backoff.Retry(func() error { // retry with backoff
				var err error

				d := time.Now().Add(1 * time.Minute)
				ctx, cancel := context.WithDeadline(context.Background(), d)
				defer cancel()

				resp, err = dlaasGrpcClient.Client().DeleteTrainingJob(ctx, &grpc_trainer_v2.DeleteRequest{
					TrainingId: trainingID,
					UserId:     userID,
				})
				// if trainer returns error, start to retry
				if err != nil {
					switch grpc.Code(err) {
					// if the error is NotFound, quit backoff and return no error
					case codes.NotFound:
						fmt.Println("Training ID not found.")
						return nil
						// if the error is other type, start to retry
					default:
						fmt.Println("dlaasGrpcClient.Client().DeleteTrainingJob(...) failed. Retrying...")
					}
				}
				return err
			}, defaultBackoff)
			defaultBackoff.Reset()
			if err != nil {
				fmt.Println(err.Error())
				return
			}else{
				fmt.Printf("Training model %s deleted\n", trainingID)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVarP(&deleteAll, "all", "a", false,
		"if specified, delete all training instances")
}
