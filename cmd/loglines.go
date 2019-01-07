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
	"io"
	"os"
	"time"

	"github.com/AISphere/ffdl-trainer/trainer/grpc_trainer_v2"

	"encoding/json"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultLogsPageSize       = 10
	defaultLogsFollowSleep    = time.Second * 1
	defaultWaitForEndJobCheck = time.Second * 5
)

var logsFollow = false
var logsJSON = false
var logsOutput = false
var logsTeeOutput = false
var logsSubID = ""
var logsPos int64 = 1
var logsPagesize int32 = defaultLogsPageSize

type LogLinesTimings struct {
	trainingID     string
	secondsForCall float64
	nLinesInResp   int64
	firstLineTime  float64
	lastLineTime   float64
}

// IsValidTrainingID checks on the training id to see if it is valid.  If it is
// not, report if shouldReport is true, and return false.  Otherwise return true.
//noinspection GoUnusedParameter
func IsValidTrainingID(trainingID string, userID string, shouldReport bool) bool {
	var resp *grpc_trainer_v2.GetResponse
	isValid := false
	var err error
	for i := 0; i < 4; i++ { // retry get with backoff
		d := time.Now().Add(1 * time.Minute)
		ctx, cancel := context.WithDeadline(context.Background(), d)
		defer cancel()
		resp, err = dlaasGrpcClient.Client().GetTrainingJob(ctx, &grpc_trainer_v2.GetRequest{
			TrainingId: trainingID,
			UserId:     userID,
		})
		if err != nil {
			if statusObj, ok := status.FromError(err); ok {
				switch //noinspection GoDeprecation
				statusObj.Code() {
				// if the error is NotFound, quit backoff and return no error
				case codes.NotFound:
					fmt.Println("Training ID not found, please check if your id is valid")
					logsFollow = false
					break
					// if the error is other type, start to retry
				default:
					fmt.Printf("GetTrainingJob(...) failed (%s). Retrying..\n",
						statusObj.Message())
				}
			} else {
				fmt.Printf("unknown error")
			}
		} else {
			isValid = true
			break
		}
	}

	if resp == nil || err != nil {
		isValid = false
	}

	return isValid
}

func llGetTimeOfPos(trainingID string, userID string, pos int64) int64 {

	var nanoTimeFound int64 = max64BitInt

	query := &grpc_trainer_v2.Query{
		Meta: &grpc_trainer_v2.MetaInfo{
			TrainingId: trainingID,
			UserId:     userID,
			Subid:      logsSubID,
		},
		Pos:      pos,
		Pagesize: logsPagesize,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*4)
	defer cancel()

	var inStream grpc_trainer_v2.Trainer_GetTrainingLogsClient
	trainerService := dlaasGrpcClient.Client()
	for {
		var err error
		for i := 0; i < 4; i++ { // retry
			inStream, err = trainerService.GetTrainingLogs(ctx, query)
			if err != nil {
				if statusObj, ok := status.FromError(err); ok {
					fmt.Printf("error: %s", statusObj.Details())
				} else {
					fmt.Printf("unknown error")
				}
			} else if inStream != nil {
				break
			}
		}
		if (err != nil) || (inStream == nil) {
			fmt.Println(err.Error())
			break
		}

		var nLinesInResp int64

		for {
			chunk, err := inStream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Printf("cannot read trained model log: %v\n", err)
				break
			}
			nLinesInResp++
			if chunk.Meta.Time < nanoTimeFound {
				nanoTimeFound = chunk.Meta.Time
			}
		}
		if nLinesInResp > 0 {
			break
		}
		time.Sleep(time.Second)
	}
	return nanoTimeFound
}

func fetchLogLines(trainingID string, isScaleTest bool,
	timingsList []*LogLinesTimings, logLineTimingChan chan *LogLinesTimings) []*LogLinesTimings {

	userID := os.Getenv("DLAAS_USERID")
	if userID == "" {
		fmt.Println("`DLAAS_USERID` is not set")
		return timingsList
	}
	if initTrainerClient() {
		trainerService := dlaasGrpcClient.Client()

		if !IsValidTrainingID(trainingID, userID, true) {
			return timingsList
		}

		totalLines := 0
		outputPath := trainingID + ".log"
		lastReceiveTime := time.Now()

		var nanoTime int64
		if logsPos == 1 {
			nanoTime = time.Date(1678, 1,
				1, 0, 0, 0, 0, time.Local).UnixNano()
		} else {
			nanoTime = llGetTimeOfPos(trainingID, userID, logsPos)
		}

		query := &grpc_trainer_v2.Query{
			Meta: &grpc_trainer_v2.MetaInfo{
				TrainingId: trainingID,
				UserId:     userID,
				Time:       nanoTime,
				Subid:      logsSubID,
			},
			Pagesize: logsPagesize,
		}

		for {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			//noinspection GoDeferInLoop
			defer cancel()

			isEmptyLines := false
			var inStream grpc_trainer_v2.Trainer_GetTrainingLogsClient
			beforeCall := time.Now()
			var err2 error
			for i := 0; i < 4; i++ { // retry with backoff
				inStream, err2 = trainerService.GetTrainingLogs(ctx, query)
				if err2 != nil {
					if statusObj, ok := status.FromError(err2); ok {
						fmt.Printf("error: %s", statusObj.Details())
					} else {
						fmt.Printf("unknown error")
					}
				} else if inStream != nil {
					break
				}
			}
			if err2 != nil {
				break
			} else if inStream == nil {
				continue
			}

			afterCall := time.Now()

			var nLinesInResp int64 = 0

			var firstLineTime = 0.0
			var lastLineTime = 0.0
			var lastLineSentTime int64

			for {
				chunk, err := inStream.Recv()
				if err == io.EOF {
					if nLinesInResp == 0 {
						isEmptyLines = true
					}
					break
				}
				if err != nil {
					fmt.Printf("cannot read trained model log: %v\n", err)
					break
				}
				if chunk == nil {
					break
				}
				lastReceiveTime = time.Now()

				var outputLine string

				if !isScaleTest || logsOutput {
					if logsJSON {
						jsonBytes, err := json.Marshal(chunk)
						if err != nil {
							fmt.Printf("Could not marshal record to json: %s", err.Error())
						}
						outputLine = fmt.Sprintf("%s\n", string(jsonBytes))
					} else {
						outputLine = fmt.Sprintf("%v %v %s", chunk.Meta.Rindex, chunk.Meta.Time, chunk.Line)
					}
				}

				if !isScaleTest && (!logsOutput || logsTeeOutput) {
					fmt.Printf("%s", outputLine)
				}

				if logsOutput {
					logsFollow = true
					f, err := os.OpenFile(outputPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)

					if err != nil {
						fmt.Printf("Path %s is invalid. Can't write file.\n", outputPath)
						return timingsList
					}
					_, err = f.Write([]byte(outputLine))
					if err != nil {
						panic(err)
					}
					f.Close()
				}

				if nLinesInResp == 0 {
					timeSent := float64(chunk.Meta.Time) / 1000
					timeArrived := time.Duration(afterCall.UnixNano()).Seconds()
					firstLineTime = timeArrived - timeSent
				}
				lastLineSentTime = chunk.Meta.Time

				nLinesInResp++
				totalLines++

				query = &grpc_trainer_v2.Query{
					Meta: &grpc_trainer_v2.MetaInfo{
						TrainingId: trainingID,
						UserId:     userID,
						Time:       chunk.Meta.Time + 1,
						Subid:      logsSubID,
					},
					Pagesize: logsPagesize,
				}
			}

			if nLinesInResp > 0 {
				if nLinesInResp > 0 {
					timeSent := float64(lastLineSentTime) / 1000
					timeArrived := time.Duration(afterCall.UnixNano()).Seconds()
					lastLineTime = timeArrived - timeSent
				}

				if isScaleTest {
					secondsForCall := afterCall.Sub(beforeCall).Seconds()

					rec := LogLinesTimings{
						trainingID,
						secondsForCall,
						nLinesInResp,
						firstLineTime,
						lastLineTime,
					}

					timingsList = append(timingsList, &rec)
					logLineTimingChan <- &rec
				}
			}

			durationWait := time.Now().Sub(lastReceiveTime)
			if durationWait > defaultWaitForEndJobCheck {
				var resp *grpc_trainer_v2.GetResponse
				var err3 error
				for i := 1; i < 4; i++ { // retry get with backoff
					d := time.Now().Add(30 * time.Second)
					ctx, cancel := context.WithDeadline(context.Background(), d)
					defer cancel()
					resp, err3 = dlaasGrpcClient.Client().GetTrainingJob(ctx, &grpc_trainer_v2.GetRequest{
						TrainingId: trainingID,
						UserId:     userID,
					})
					if err3 != nil {
						if err3 != nil {
							if statusObj, ok := status.FromError(err3); ok {
								switch //noinspection GoDeprecation
								statusObj.Code() {
								// if the error is NotFound, quit backoff and return no error
								case codes.NotFound:
									fmt.Println("Training ID not found, please check if your id is valid")
									return nil
									// if the error is other type, start to retry
								default:
									fmt.Printf("GetTrainingJob(...) failed (%s). Retrying..\n",
										statusObj.Message())
								}
							} else {
								fmt.Printf("unknown error")
							}
						}
					}
				}

				if resp != nil && resp.Job != nil && resp.Job.Status != nil {
					if resp.Job.Status.Status == grpc_trainer_v2.Status_FAILED ||
						resp.Job.Status.Status == grpc_trainer_v2.Status_HALTED ||
						resp.Job.Status.Status == grpc_trainer_v2.Status_COMPLETED {

						if !isScaleTest {
							fmt.Printf("Job completion status: %s, %s\n", resp.Job.Status.Status.String(),
								resp.Job.Status.StatusMessage)
						}
						break
					}
				} else {
					logsFollow = false
					break
				}
			}
			if !logsFollow {
				break
			}
			if isEmptyLines {
				time.Sleep(defaultLogsFollowSleep)
			}
		}
		if logsOutput {
			fmt.Printf("Saved model log in %s\nWrote %d lines\n", outputPath, totalLines)
		}
	}
	return timingsList
}

// logsCmd represents the logs command
var loglinesCmd = &cobra.Command{
	Use:     "loglines TRAINING_ID",
	Aliases: []string{"logs", "log"},
	Short:   "View the ongoing training logs",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			fmt.Println("Incorrect usage\ndlaas logs -h for more details")
			return
		}
		var trainingID = args[0]
		fetchLogLines(trainingID, false, nil, nil)
	},
}

func init() {
	rootCmd.AddCommand(loglinesCmd)
	loglinesCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false,
		"if specified, follow the log")
	loglinesCmd.Flags().BoolVarP(&logsJSON, "json", "j", false,
		"if specified, logsOutput logs as json")
	loglinesCmd.Flags().Int64VarP(&logsPos, "pos", "p", 1,
		"If positive, line number from start, if negative, line position from end")
	loglinesCmd.Flags().Int32VarP(&logsPagesize, "pagesize", "s", defaultLogsPageSize,
		"Number of lines to deliver")
	loglinesCmd.Flags().BoolVarP(&logsOutput, "logsOutput", "o", false,
		"if specified, logsOutput log as TRAINING_ID.log to deep-learning-platform/dlaas-user-guide/")
	loglinesCmd.Flags().BoolVarP(&logsTeeOutput, "tee", "t", false,
		"if specified and logsOutput is also specified, write to stdout also")
	loglinesCmd.Flags().StringVarP(&logsSubID, "subid", "i", "",
		"if specified use subid as qualifier")
}
