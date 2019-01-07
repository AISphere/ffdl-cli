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
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/AISphere/ffdl-trainer/trainer/grpc_trainer_v2"

	"github.com/cenkalti/backoff"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// downloadCmd represents the download command
var downloadCmd = &cobra.Command{
	Use:   "download TRAINING_ID ",
	Short: "Download the trained model to local",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			fmt.Println("Incorrect arguments\nffdl download -h for more details")
			return
		}
		userID := os.Getenv("DLAAS_USERID")
		if userID == "" {
			fmt.Println("`DLAAS_USERID` is not set")
			return
		}
		if initTrainerClient() {
			trainingID := args[0]

			var buffer *bytes.Buffer
			var path string

			data := make(map[bool]interface{})
			data[true] = definition


			var err error

			if definition {
				//============download model definition is 10===============
				fmt.Printf("Downloading model definition '%s' ...\n", trainingID)
				var resp grpc_trainer_v2.Trainer_GetModelDefinitionClient
				d := time.Now().Add(10 * time.Minute)
				ctx, cancel := context.WithDeadline(context.Background(), d)
				defer cancel()
				err = backoff.Retry(func() error { // retry with backoff
					buffer = new(bytes.Buffer)
					resp, err = dlaasGrpcClient.Client().GetModelDefinition(ctx, &grpc_trainer_v2.ModelDefinitionRequest{
						TrainingId: trainingID,
						UserId:     userID,
					})
					if err != nil {
						switch grpc.Code(err) {
						case codes.NotFound:
							fmt.Println("Training ID not found.")
							return nil
						default:
							fmt.Println("dlaasGrpcClient.Client().GetModelDefinition(...) failed. Retrying...")
						}
					}
					return err
				}, defaultBackoff)

				for {
					chunk, err := resp.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						break
					}
					if chunk != nil {
						buffer.Write(chunk.Data)
					}
				}
				defaultBackoff.Reset()
				if err != nil {
					fmt.Println(err.Error())
				}
				if filename == "" {
					path = trainingID + "_definition.zip"
				} else {
					path = filename + ".zip"
				}
				err = ioutil.WriteFile(path, buffer.Bytes(), 0644)
				if err != nil {
					fmt.Printf("Path %s is invalid\n", path)
					return
				}
				if buffer.Bytes() == nil {
					fmt.Println("The downloaded zip is empty, please make sure your training ID is valid.")
				} else {
					fmt.Printf("Model definition %s is downloaded to %s successfully\n", trainingID, path)
				}
				return
				//============download model definition is 10===============
			} else {
				//============download trained model is 5===============
				fmt.Printf("Downloading trained model '%s' ...\n", trainingID)
				var resp grpc_trainer_v2.Trainer_GetTrainedModelClient
				d := time.Now().Add(5 * time.Minute)
				ctx, cancel := context.WithDeadline(context.Background(), d)
				defer cancel()

				err = backoff.Retry(func() error { // retry with backoff
					buffer = new(bytes.Buffer)
					resp, err = dlaasGrpcClient.Client().GetTrainedModel(ctx, &grpc_trainer_v2.TrainedModelRequest{
						TrainingId: trainingID,
						UserId:     userID,
					})
					// if trainer returns error, start to retry
					if err != nil {
						switch grpc.Code(err) {
						// if the error is NotFound, quit backoff and return no error
						case codes.NotFound:
							fmt.Printf("Training ID not found")
							return nil
						// if the error is other type, start to retry
						default:
							fmt.Printf("dlaasGrpcClient.Client().GetTrainedModel(...) failed. Retrying...\n")
						}
					}
					return err
				}, defaultBackoff)
				if filename == "" {
					path = trainingID + "_trainedmodel.zip"
				} else {
					path = filename + ".zip"
				}
				for {
					chunk, err := resp.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						break
					}
					if chunk != nil {
						buffer.Write(chunk.Data)
					}
				}
				defaultBackoff.Reset()
				if err != nil {
					fmt.Println(err.Error())
				}
				err = ioutil.WriteFile(path, buffer.Bytes(), 0644)
				if err != nil {
					fmt.Printf("Path %s is invalid\n", path)
					return
				}
				if buffer.Bytes() == nil {
					fmt.Println("The downloaded zip is empty")
				} else {
					fmt.Printf("Trained model %s is downloaded to %s successfully\n", trainingID, path)
				}
				return
				//============download trained model is 5===============
			}
		}
	},
}

var definition bool
var filename string

func init() {
	rootCmd.AddCommand(downloadCmd)
	downloadCmd.Flags().BoolVarP(&definition, "definition", "d", false, "download the model definition")
	downloadCmd.Flags().StringVarP(&filename, "filename", "f", "", "define downloaded file name")
}
