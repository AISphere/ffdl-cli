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
	"time"

	"github.com/cenkalti/backoff"
	"github.com/spf13/cobra"
	"github.com/AISphere/ffdl-trainer/trainer/grpc_trainer_v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status TRAINING_ID",
	Short: "Show model training status",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			fmt.Println("Incorrect usage\ndlaas status -h for more details")
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

			fmt.Printf("Getting training status ID '%s' ...\n", trainingID)

			var resp *grpc_trainer_v2.GetStatusIDResponse

			d := time.Now().Add(1 * time.Minute)
			ctx, cancel := context.WithDeadline(context.Background(), d)
			defer cancel()
			err := backoff.Retry(func() error { // retry with backoff
				var err error
				resp, err = dlaasGrpcClient.Client().GetTrainingStatusID(ctx, &grpc_trainer_v2.GetRequest{
					TrainingId: trainingID,
					UserId:     userID,
				})
				// if trainer returns error, start to retry
				if err != nil {
					switch grpc.Code(err) {
					// if the error is NotFound, quit backoff and return no error
					case codes.NotFound:
						fmt.Println("Training ID not found")
						return nil
					// if the error is other type, start to retry
					default:
						fmt.Println("Getting training status ID failed. Retrying..")
					}
				}
				return err
			}, defaultBackoff)
			defaultBackoff.Reset()
			if err != nil {
				fmt.Println(err.Error())
				fmt.Printf("Couldn't get training model status...\n")
			}
			if resp != nil {
				switch os := resp.Status; os {
				case 0:
					fmt.Println("Training Status is Status_NOT_STARTED")
				case 1:
					fmt.Println("Training Status is Status_PENDING")
				case 5:
					fmt.Println("Training Status is Status_HALTED")
				case 10:
					fmt.Println("Training Status is Status_FAILED")
				case 20:
					fmt.Println("Training Status is Status_DEPLOY")
				case 30:
					fmt.Println("Training Status is Status_DOWNLOADING")
				case 40:
					fmt.Println("Training Status is Status_PROCESSING")
				case 50:
					fmt.Println("Training Status is Status_STORING")
				case 60:
					fmt.Println("Training Status is Status_COMPLETED")
				case 70:
					fmt.Println("Training Status is Status_QUEUED")
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
