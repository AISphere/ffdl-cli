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

// haltCmd represents the halt command
var haltCmd = &cobra.Command{
	Use:   "halt TRAINING_ID",
	Short: "Halt a training Job (not implemented)",
	Run: func(cmd *cobra.Command, args []string) {
		initTrainerClient()
		if len(args) != 1 {
			fmt.Println("Incorrect arguments\nffdl halt -h for more details")
			return
		}
		var resp *grpc_trainer_v2.HaltResponse
		userID := os.Getenv("DLAAS_USERID")
		if userID == "" {
			fmt.Println("`DLAAS_USERID` is not set")
			return
		}

		trainingID := args[0]
		if !IsValidTrainingID(trainingID, userID, true) {
			return
		}
		fmt.Printf("Halting training Job '%s' ...", trainingID)
		d := time.Now().Add(1 * time.Minute)
		ctx, cancel := context.WithDeadline(context.Background(), d)
		defer cancel()

		err := backoff.Retry(func() error { // retry with backoff
			var err error
			resp, err = dlaasGrpcClient.Client().HaltTrainingJob(ctx, &grpc_trainer_v2.HaltRequest{
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
					fmt.Println("dlaasGrpcClient.Client().HaltTrainingJob(...) failed. Retrying...")
				}
			}
			return err
		}, defaultBackoff)
		defaultBackoff.Reset()
		if err != nil {
			fmt.Println(err.Error())
		}
		if resp != nil {
			switch os := resp.Status; os {
			case 0:
				fmt.Println("\nTraining Status is Status_NOT_STARTED")
			case 1:
				fmt.Println("\nTraining Status is Status_PENDING")
			case 5:
				fmt.Println("\nTraining Status is Status_HALTED")
			case 10:
				fmt.Println("\nTraining Status is Status_FAILED")
			case 20:
				fmt.Println("\nTraining Status is Status_DEPLOY")
			case 30:
				fmt.Println("\nTraining Status is Status_DOWNLOADING")
			case 40:
				fmt.Println("\nTraining Status is Status_PROCESSING")
			case 50:
				fmt.Println("\nTraining Status is Status_STORING")
			case 60:
				fmt.Println("\nTraining Status is Status_COMPLETED")
			}
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(haltCmd)
}
