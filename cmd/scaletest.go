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
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/AISphere/ffdl-trainer/trainer/grpc_trainer_v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var scaleTestVerbose = false

type AverageTimings struct {
	numberConcurrentJobs int
	secondsForQueryResponse	float64
	secondsSinceSend	float64
}

func deleteJob(training_id string, user_id string) {
	d := time.Now().Add(30 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()
	for j := 0; j < 4; j++ {
		_, err := dlaasGrpcClient.Client().DeleteTrainingJob(ctx, &grpc_trainer_v2.DeleteRequest{
			TrainingId: training_id,
			UserId:     user_id,
		})
		// if trainer returns error, start to retry
		if err != nil {
			switch grpc.Code(err) {
			// if the error is NotFound, quit backoff and return no error
			case codes.NotFound:
				fmt.Println("DeleteTrainingJob: Training ID not found.")
				break
				// if the error is other type, start to retry
			default:
				fmt.Println("DeleteTrainingJob(...) failed.")
			}
		}
	}

}

// scaleTest represents the scale-test command
var scaleTest = &cobra.Command{
	Use:   "scale-test MANIFEST_PATH MODEL_PATH [iter]+",
	Short: "Scale testing",
	Run: func(cmd *cobra.Command, args []string) {
		initTrainerClient()
		userID := os.Getenv("DLAAS_USERID")
		if userID == "" {
			fmt.Println("`DLAAS_USERID` is not set")
			return
		}
		argPos := 0
		manifestPath := args[argPos]
		argPos++
		modelPath := args[argPos]
		argPos++

		scaleEpochs := make([]int, 0)

		for i := argPos; i < len(args); i++ {
			numberIterations, err := strconv.Atoi(args[argPos])
			argPos++
			if err != nil {
				fmt.Printf("argument %d must specify iteration count\n", argPos)
				return
			}
			scaleEpochs = append(scaleEpochs, numberIterations)
		}
		nEpochs := len(scaleEpochs)

		manifest, err := os.Open(manifestPath)
		if err != nil {
			fmt.Println("Could not load manifest file")
			return
		}
		var modelDefinition *os.File
		if strings.Contains(modelPath, ".zip") {
			modelDefinition, err = os.Open(modelPath)
		} else {
			modelDir := args[1]
			f, err2 := zipit(modelDir + "/")
			if err2 != nil {
				fmt.Printf("Unexpected error when compressing model directory: %v", err2)
				os.Exit(1)
			}

			// reopen the file (I tried to leave the file open but it did not work)
			modelDefinition, err = os.Open(f.Name())
			if err != nil {
				fmt.Println("Error open temporary ZIP file: ", err)
			}
			defer os.Remove(modelDefinition.Name())
		}

		if err != nil {
			fmt.Println("Could not load model definition")
			return
		}
		//Convert manifest file to a Training Request
		manifestReader := bufio.NewReader(manifest)
		manifestBytes, err := ioutil.ReadAll(manifestReader)
		if err != nil {
			panic(err)
		}
		mConfig, err := loadManifestV1(manifestBytes)
		if err != nil {
			panic(err)
		}

		modelDefReader := bufio.NewReader(modelDefinition)
		modelDefBytes, err := ioutil.ReadAll(modelDefReader)

		if err != nil {
			panic(err)
		}
		//Convert manifest file to a Training Request
		createReq := manifest2TrainingRequest(mConfig, modelDefBytes)

		averagesLLList := make([]*AverageTimings, nEpochs)
		averagesEMList := make([]*AverageTimings, nEpochs)

		startEpochs := time.Now()

		didPrintColumnHeaders := false

		for epochIndex, numberIterations := range scaleEpochs {
			startThisEpoch := time.Now()

			fmt.Printf("\n-----\nstarting epoch %d, launching %d concurrent instances\n",
				epochIndex, numberIterations)

			var trainingIDs []string

			allLogLinesDone := true
			allEmetricsDone := true

			nLLTargetIterations := 0
			if logsFollow {
				nLLTargetIterations = numberIterations
				allLogLinesDone = false
			}
			nEMTargetIterations := 0
			if emetricsFollow {
				nEMTargetIterations = numberIterations
				allEmetricsDone = false
			}

			numLogLineTimingDoneChan := make(chan int, nLLTargetIterations)
			numEMetricsTimingDoneChan := make(chan int, nEMTargetIterations)

			var logLinesTimings= make([]*LogLinesTimings, 0)
			var eMetricsTimings= make([]*EMetricsTimings, 0)

			for i := 0; i < numberIterations; i++ {
				//d := time.Now().Add(10 * time.Second)
				//ctx, cancel := context.WithDeadline(context.Background(), d)
				//defer cancel()
				ctx := context.Background()

				var resp *grpc_trainer_v2.CreateResponse
				resp, err = dlaasGrpcClient.Client().CreateTrainingJob(ctx, createReq)

				if err != nil {
					fmt.Printf("Error:\n\t")
					fmt.Println(err.Error())
					fmt.Println("dlaasGrpcClient.Client().CreateTrainingJob(...) returned error. Giving up.")
				}
				if resp != nil {

					trainingIDs = append(trainingIDs, resp.TrainingId)

					if scaleTestVerbose {
						fmt.Printf("Training job created. Training ID is %s\n", resp.TrainingId)
					} else if !logsFollow && !emetricsFollow {
						fmt.Print("T")
					}

					logLineTimingDone := make(chan int, 0)
					eMetricsTimingDone := make(chan int, 0)

					logLineTimingChan := make(chan *LogLinesTimings)
					eMetricsTimingChan := make(chan *EMetricsTimings)

					logLinesDone := false
					emetricsDone := false

					if logsFollow {
						go func() {
							logLinesTimings = fetchLogLines(resp.TrainingId, true,
								logLinesTimings, logLineTimingChan)
							logLineTimingDone <- 1
						}()
					} else {
						logLinesDone = true
					}

					if emetricsFollow {
						go func() {
							eMetricsTimings = fetchEMetrics(resp.TrainingId, true,
								eMetricsTimings, eMetricsTimingChan)
							eMetricsTimingDone <- 1
						}()
					} else {
						emetricsDone = true
					}

					go func() {
						if didPrintColumnHeaders == false {
							didPrintColumnHeaders = true
							fmt.Printf("\n=============\n")
							fmt.Printf("%5s\t%3s\t%19s\t%9s\t%3s\t%5s\t%5s\n",
								"Epoch", "Typ", "TID", "LLQRsp", "NRcv", "LLSnd", "FLSnd")
						}
						for ; !logLinesDone || !emetricsDone; {
							select {
							case isDone := <-logLineTimingDone:
								if isDone == 1 {
									if scaleTestVerbose {
										fmt.Printf("Marking done LL: %s\n", resp.TrainingId)
									}
									if emetricsDone {
										deleteJob(resp.TrainingId, userID)
									}
									numLogLineTimingDoneChan <- 1
									logLinesDone = true
								}
							case isDone := <-eMetricsTimingDone:
								if isDone == 1 {
									if scaleTestVerbose {
										fmt.Printf("Marking done EM: %s\n", resp.TrainingId)
									}
									if logLinesDone {
										deleteJob(resp.TrainingId, userID)
									}
									numEMetricsTimingDoneChan <- 1
									emetricsDone = true
								}
							case rec := <-logLineTimingChan:
								fmt.Printf("%5d\t%3s\t%19s\t%09.6f\t%3d\t%05.03f\t%05.03f\n",
									epochIndex, "LL", rec.trainingID, rec.secondsForCall, rec.nLinesInResp,
									rec.firstLineTime, rec.lastLineTime)

							case rec := <-eMetricsTimingChan:
								fmt.Printf("%5d\t%3s\t%19s\t%09.6f\t%3d\t%05.03f\t%05.03f\n",
									epochIndex, "EM", rec.trainingID, rec.secondsForCall, rec.nLinesInResp,
									rec.firstLineTime, rec.lastLineTime)

							case <-time.After(20 * time.Minute):
								fmt.Print("Time out waiting for the logs and/or emetrics (training level)!\n")
								logLinesDone = true
								emetricsDone = true
								deleteJob(resp.TrainingId, userID)
								numEMetricsTimingDoneChan <- 1
								break
							}
						}
					}()
				} else {
					if logsFollow {
						nLLTargetIterations--
					}
					if emetricsFollow {
						nEMTargetIterations--
					}
				}
			}

			if scaleTestVerbose {
				fmt.Print("Waiting for all loops to complete!\n")
			}
			for ; !allLogLinesDone || !allEmetricsDone; {
				select {
				case  <-numLogLineTimingDoneChan:
					nLLTargetIterations -= 1
					if scaleTestVerbose {
						fmt.Printf("A job in epoch LL %d is done, still running instances: %d\n",
							epochIndex, nLLTargetIterations)
					}
					if nLLTargetIterations <= 0 {
						allLogLinesDone = true
					}
				case  <-numEMetricsTimingDoneChan:
					nEMTargetIterations -= 1
					if scaleTestVerbose {
						fmt.Printf("A job in epoch EM %d is done, still running instances: %d\n",
							epochIndex, nEMTargetIterations)
					}
					if nEMTargetIterations <= 0 {
						allEmetricsDone = true
					}
				case <-time.After(40 * time.Minute):
					fmt.Printf("Time out waiting for the logs and/or emetrics (epoch level): %d left\n",
						nEMTargetIterations)
					allLogLinesDone = true
					allEmetricsDone = true
					break
				}
			}

			averageLLTimes := AverageTimings{
				numberConcurrentJobs: numberIterations,
				secondsForQueryResponse: 0.0,
				secondsSinceSend:        0.0,
			}
			averageEMTimes := AverageTimings{
				numberConcurrentJobs: numberIterations,
				secondsForQueryResponse: 0.0,
				secondsSinceSend:        0.0,
			}

			if logLinesTimings != nil && len(logLinesTimings) > 0 {
				for _, rec := range logLinesTimings {
					averageLLTimes.secondsForQueryResponse += rec.secondsForCall
					averageLLTimes.secondsSinceSend += rec.firstLineTime
					averageLLTimes.secondsSinceSend += rec.lastLineTime
				}
				averageLLTimes.secondsForQueryResponse =
					averageLLTimes.secondsForQueryResponse / float64(len(logLinesTimings))
				averageLLTimes.secondsSinceSend = averageLLTimes.secondsSinceSend / float64(len(logLinesTimings)*2)
			}
			if eMetricsTimings != nil && len(eMetricsTimings) > 0 {
				for _, rec := range eMetricsTimings {
					averageEMTimes.secondsForQueryResponse += rec.secondsForCall
					averageEMTimes.secondsSinceSend += rec.firstLineTime
					averageEMTimes.secondsSinceSend += rec.lastLineTime
				}
				averageEMTimes.secondsForQueryResponse =
					averageEMTimes.secondsForQueryResponse / float64(len(eMetricsTimings))
				averageEMTimes.secondsSinceSend = averageEMTimes.secondsSinceSend / float64(len(eMetricsTimings)*2)
			}
			averagesLLList[epochIndex] = &averageLLTimes
			averagesEMList[epochIndex] = &averageEMTimes

			fmt.Print("\n#####\n")
			fmt.Printf("Averages for Epoch %d (%d iterations) results:\n", epochIndex, numberIterations)
			fmt.Printf("\n%5s\t%9s\t%9s\n", "NConc", "LLQRsp", "EMQRsp")
			fmt.Printf("%5d\t%09.6f\t%09.6f\n",
				numberIterations,
				averagesLLList[epochIndex].secondsForQueryResponse,
				averagesEMList[epochIndex].secondsForQueryResponse)

			fmt.Printf("\n%5s\t%9s\t%9s\n", "NConc", "LLSnd", "EMSnd")
			fmt.Printf("%5d\t%09.6f\t%09.6f\n",
				numberIterations,
				averagesLLList[epochIndex].secondsSinceSend,
				averagesEMList[epochIndex].secondsSinceSend)
			fmt.Print("\n#####\n")

			elapsedThisEpoch := time.Since(startThisEpoch)

			if scaleTestVerbose {
				fmt.Printf("\nDone with Epoch %d, %d jobs, Total time: %4.2f minutes\n",
					epochIndex, numberIterations, elapsedThisEpoch.Minutes())
			}
			time.Sleep(5*time.Second)
		}
		timeForAllEpochs := time.Since(startEpochs)

		fmt.Print("\n---------------------------------\n")
		fmt.Print("Averages keys:\n")
		fmt.Print("NConc: Number of concurent instances\n")
		fmt.Print("LLQRsp: Average Logline Query Response Time (10 or less records)\n")
		fmt.Print("LLSnd: Average Logline Time since log-collector send\n")
		fmt.Print("EMQRsp: Average EMetrics Query Response Time (10 or less records\n")
		fmt.Print("LLSnd: Average EMetrics Time since log-collector send\n")
		fmt.Print("---------------------------------\n")

		fmt.Printf("\n%5s\t%9s\t%9s\n", "NConc", "LLQRsp", "EMQRsp")
		for epochIndex, numberIterations := range scaleEpochs {
			fmt.Printf("%5d\t%09.6f\t%09.6f\n",
				numberIterations,
				averagesLLList[epochIndex].secondsForQueryResponse,
				averagesEMList[epochIndex].secondsForQueryResponse)
		}

		fmt.Printf("\n%5s\t%9s\t%9s\n", "NConc", "LLSnd", "EMSnd")
		for epochIndex, numberIterations := range scaleEpochs {
			fmt.Printf("%5d\t%09.6f\t%09.6f\n",
				numberIterations,
				averagesLLList[epochIndex].secondsSinceSend,
				averagesEMList[epochIndex].secondsSinceSend)
		}


		if scaleTestVerbose {
			fmt.Printf("\nDONE! Total time: %4.2f minutes\n", timeForAllEpochs.Minutes())
		}

		return
	},
}

func init() {
	rootCmd.AddCommand(scaleTest)
	// -f and -l are synonyms
	scaleTest.Flags().BoolVarP(&scaleTestVerbose, "verbose", "v", false,
		"if specified, be annoyingly verbose")
	scaleTest.Flags().BoolVarP(&logsFollow, "logs", "l", false,
		"if specified, follow the log")
	scaleTest.Flags().BoolVarP(&emetricsFollow, "emetrics", "e", false,
		"if specified, follow the emetrics")
	scaleTest.Flags().BoolVarP(&logsOutput, "logsOutput", "o", false,
		"if specified, logsOutput log or emetrics as TRAINING_ID.log to deep-learning-platform/dlaas-user-guide/")
	scaleTest.Flags().BoolVarP(&logsTeeOutput, "tee", "t", false,
		"if specified and logs or emetrics is also specified, write to stdout also")
	scaleTest.Flags().BoolVarP(&logsJSON, "json", "j", false,
		"if specified, logsOutput logs as json")
}
