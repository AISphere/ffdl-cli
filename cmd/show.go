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
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/AISphere/ffdl-trainer/trainer/grpc_trainer_v2"

	"github.com/cenkalti/backoff"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show TRAINING_ID",
	Short: "Get detailed information about models",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			fmt.Println("Incorrect usage\nffdl show -h for more details")
			return
		}
		if initTrainerClient() {
			modelID := args[0]
			fmt.Printf("Querying model with ID: %s ...\n", modelID)
			userID := os.Getenv("DLAAS_USERID")
			if userID == "" {
				fmt.Println("`DLAAS_USERID` is not set")
				return
			}
			if !IsValidTrainingID(modelID, userID, true) {
				return
			}

			var resp *grpc_trainer_v2.GetResponse
			err := backoff.Retry(func() error { // retry get with backoff
				var err error
				d := time.Now().Add(1 * time.Minute)
				ctx, cancel := context.WithDeadline(context.Background(), d)
				defer cancel()
				resp, err = dlaasGrpcClient.Client().GetTrainingJob(ctx, &grpc_trainer_v2.GetRequest{
					TrainingId: modelID,
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
						fmt.Println("dlaasGrpcClient.Client().GetTrainingJob(...) failed. Retrying..")
					}
				}
				return err
			}, defaultBackoff)
			defaultBackoff.Reset()
			if resp != nil {
				data, err := json.MarshalIndent(resp.GetJob(), "", "  ")
				if err != nil {
					fmt.Println("Training detail is not in JSON format")
				}
				data2 := append(data, '\n')
				fmt.Printf("%s\n", data2)

				return
			}
			if err != nil {
				fmt.Println(err.Error())
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
