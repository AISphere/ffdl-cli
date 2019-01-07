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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"archive/zip"
	"io"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/AISphere/ffdl-trainer/trainer/grpc_trainer_v2"
	yaml "gopkg.in/yaml.v2"
)

func zipit(source string) (*os.File, error) {
	// on windows we are seeing the source path in the ZIP, so we change the current
	// dir as a workaround and reset is back after
	if runtime.GOOS == "windows" {
		currentDir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		defer os.Chdir(currentDir)
		source, _ = filepath.Abs(source)
		if err := os.Chdir(source); err != nil {
			return nil, err
		}
	}
	zipfile, err := ioutil.TempFile("", "dlaas")
	if err != nil {
		return nil, err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil, err
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if source != path {
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}

			if baseDir != "" {
				header.Name = strings.TrimPrefix(path, source)
			}

			if info.IsDir() {
				header.Name += "/"
			} else {
				header.Method = zip.Deflate
			}

			writer, err := archive.CreateHeader(header)

			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(writer, file)
			return err
		}
		return err
	})

	return zipfile, nil
}

// trainCmd represents the train command
var trainCmd = &cobra.Command{
	Use:   "train MANIFEST_PATH MODEL_PATH",
	Short: "Start training a model",
	Run: func(cmd *cobra.Command, args []string) {
		initTrainerClient()
		if len(args) != 2 {
			fmt.Println("Incorrect arguments\nffdl train -h for more details")
			return
		}
		fmt.Printf("Creating training job...\n")
		userID := os.Getenv("DLAAS_USERID")
		if userID == "" {
			fmt.Println("`DLAAS_USERID` is not set")
			return
		}
		manifestPath := args[0]
		modelPath := args[1]
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

		fmt.Println(modelPath)
		if err != nil {
			fmt.Println("Could not load model definition")
			return
		}
		var resp *grpc_trainer_v2.CreateResponse
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
		fmt.Println("==========")
		aaa, err := json.MarshalIndent(mConfig, "", " ")
		if err != nil {
			panic(err)
		}
		aaa2 := append(aaa, '\n')
		fmt.Printf("%s\n", aaa2)
		fmt.Println("==========")
		modelDefReader := bufio.NewReader(modelDefinition)
		modelDefBytes, err := ioutil.ReadAll(modelDefReader)

		if err != nil {
			panic(err)
		}
		//Convert manifest file to a Training Request
		createReq := manifest2TrainingRequest(mConfig, modelDefBytes)
		d := time.Now().Add(10 * time.Second)
		ctx, cancel := context.WithDeadline(context.Background(), d)
		defer cancel()
		resp, err = dlaasGrpcClient.Client().CreateTrainingJob(ctx, createReq)

		if err != nil {
			fmt.Printf("Error:\n\t")
			fmt.Println(err.Error())
			fmt.Println("dlaasGrpcClient.Client().CreateTrainingJob(...) returned error. Giving up.")
		}
		if resp != nil {
			fmt.Printf("Training job created. Training ID is %s\n", resp.TrainingId)

			if logsFollow || emetricsFollow {
				args := []string{resp.TrainingId}
				if logsFollow {
					loglinesCmd.Run(cmd, args)
				} else if emetricsFollow {
					emetricsCmd.Run(cmd, args)
				}

			}

		}
		return
	},
}

type manifestV1 struct {
	Name              string            `yaml:"name,omitempty"`
	Description       string            `yaml:"description,omitempty"`
	Version           string            `yaml:"version,omitempty"`
	Cpus              float64           `yaml:"cpus,omitempty"`
	Gpus              float64           `yaml:"gpus,omitempty"`
	GpuType           string            `yaml:"gpu_type,omitempty"`
	Learners          int32             `yaml:"learners,omitempty"`
	Memory            string            `yaml:"memory,omitempty"`
	DataStores        []*dataStoreRef   `yaml:"data_stores,omitempty"`
	Framework         *frameworkV1      `yaml:"framework,omitempty"`
	EvaluationMetrics *EMExtractionSpec `yaml:"evaluation_metrics,omitempty"`
}

// EMExtractionSpec specifies which log-collector is run, and how the evaluation metrics are extracted.
type EMExtractionSpec struct {
	Type          string              `yaml:"type,omitempty"`
	ImageTag      string              `yaml:"image_tag,omitempty"`
	In            string              `yaml:"in,omitempty"`
	LineLookahead int32               `yaml:"line_lookahead,omitempty"`
	EventTypes    []string            `yaml:"eventTypes,omitempty"`
	Groups        map[string]*EMGroup `yaml:"groups,omitempty"`
}

// EMGroup is used by the regex_extractor to specify log line matches, and the corresponding
// evaluation metrics.
type EMGroup struct {
	Regex   string          `yaml:"regex,omitempty"`
	Meta    *EMMeta         `yaml:"meta,omitempty"`
	Scalars map[string]*Any `yaml:"scalars,omitempty"`
	Etimes  map[string]*Any `yaml:"etimes,omitempty"`
}

// Any is used for typed values.
type Any struct {
	Type  string `yaml:"type,omitempty"`
	Value string `yaml:"value,omitempty"`
}

// EMMeta is used in the EMGroup record to specify the time occurrence of the evaluation metric.
type EMMeta struct {
	// Time that the metric occurred: representing the number of millisecond since midnight January 1, 1970.
	// (ref, for instance $timestamp).  Value will be extracted from timestamps
	Time string `yaml:"time,omitempty"`
}

type dataStoreRef struct {
	ID              string              `yaml:"id,omitempty"`
	Type            string              `yaml:"type,omitempty"`
	TrainingData    *storageContainerV1 `yaml:"training_data,omitempty"`
	TrainingResults *storageContainerV1 `yaml:"training_results,omitempty"`
	Connection      map[string]string   `yaml:"connection,omitempty"`
}

type frameworkV1 struct {
	Name          string           `yaml:"name,omitempty"`
	Version       string           `yaml:"version,omitempty"`
	ImageTag      string           `yaml:"image_tag,omitempty"`
	Command       string           `yaml:"command,omitempty"`
	ImageLocation *imageLocationV1 `yaml:"image_location,omitempty"`
}

type imageLocationV1 struct {
	Registry    string `yaml:"registry,omitempty"`
	Namespace   string `yaml:"namespace,omitempty"`
	AccessToken string `yaml:"access_token,omitempty"`
	Email       string `yaml:"email,omitempty"`
}

type storageContainerV1 struct {
	Container string `yaml:"container,omitempty"`
}

func loadManifestV1(data []byte) (*manifestV1, error) {
	t := &manifestV1{}
	err := yaml.Unmarshal([]byte(data), &t)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func convertMemoryFromManifest(memStr string) (float32, grpc_trainer_v2.SizeUnit, error) {
	regex, _ := regexp.Compile(`(?i)(?P<memory>(\d+)?)\s*(?P<unit>(mb|mib|gb|gib)?)`)
	match := regex.FindStringSubmatch(memStr)

	// convert to a map
	result := make(map[string]string)
	for i, name := range regex.SubexpNames() {
		if i != 0 && i <= len(match) {
			result[name] = match[i]
		}
	}
	mem, err := strconv.ParseFloat(result["memory"], 64)
	if err != nil {
		return 512, grpc_trainer_v2.SizeUnit_MB, fmt.Errorf("memory requirements not correctly specified: %s", memStr)
	}
	unit := strings.ToLower(result["unit"])

	for k, v := range grpc_trainer_v2.SizeUnit_value {
		if strings.ToLower(k) == unit {
			return float32(mem), grpc_trainer_v2.SizeUnit(v), nil
		}
	}
	return 512, grpc_trainer_v2.SizeUnit_MB, fmt.Errorf("memory requirements not correctly specified: %s", memStr)
}

func marshalToTrainerEvaluationMetricsSpec(em *EMExtractionSpec) *grpc_trainer_v2.EMExtractionSpec {
	groups := make(map[string]*grpc_trainer_v2.EMGroup)

	for key, emGroup := range em.Groups {
		values := make(map[string]*grpc_trainer_v2.EMAny)

		for scalerKey, typedValue := range emGroup.Scalars {
			marshaledValue := &grpc_trainer_v2.EMAny{Type: typedValue.Type, Value: typedValue.Value}
			values[scalerKey] = marshaledValue
		}
		etimes := make(map[string]*grpc_trainer_v2.EMAny)
		for etimeKey, typedValue := range emGroup.Etimes {
			marshaledValue := &grpc_trainer_v2.EMAny{Type: typedValue.Type, Value: typedValue.Value}
			etimes[etimeKey] = marshaledValue
		}
		group := &grpc_trainer_v2.EMGroup{
			Regex: emGroup.Regex,
			Meta: &grpc_trainer_v2.EMMeta{
				Time: emGroup.Meta.Time,
			},
			Values: values,
			Etimes: etimes,
		}
		groups[key] = group
	}

	return &grpc_trainer_v2.EMExtractionSpec{
		Type:          em.Type,
		ImageTag:      em.ImageTag,
		In:            em.In,
		LineLookahead: em.LineLookahead,
		EventTypes:    em.EventTypes,
		Groups:        groups,
	}
}

// manifest2TrainingRequest converts the existing manifest to a training request for the trainer microservice.
func manifest2TrainingRequest(m *manifestV1, modelDefinition []byte) *grpc_trainer_v2.CreateRequest {
	var userID = os.Getenv("DLAAS_USERID")
	r := &grpc_trainer_v2.CreateRequest{
		UserId: userID,
		ModelDefinition: &grpc_trainer_v2.ModelDefinition{
			Name:        m.Name,
			Description: m.Description,
			Framework: &grpc_trainer_v2.Framework{
				Name:     m.Framework.Name,
				Version:  m.Framework.Version,
				ImageTag: m.Framework.ImageTag,
			},
			Content: modelDefinition,
		},
		Training: &grpc_trainer_v2.Training{
			Command:   m.Framework.Command,
			InputData: []string{m.DataStores[0].ID + "-input"},
			Profiling: false,
		},
		Datastores: []*grpc_trainer_v2.Datastore{
			{
				Id:         m.DataStores[0].ID + "-input",
				Type:       m.DataStores[0].Type,
				Connection: m.DataStores[0].Connection,
				Fields: map[string]string{
					"bucket": m.DataStores[0].TrainingData.Container,
				},
			},
		},
	}

	if m.DataStores[0].TrainingResults != nil {
		r.Training.OutputData = []string{m.DataStores[0].ID + "-output"}
		r.Datastores = append(r.Datastores, &grpc_trainer_v2.Datastore{
			Id:         m.DataStores[0].ID + "-output",
			Type:       m.DataStores[0].Type,
			Connection: m.DataStores[0].Connection,
			Fields: map[string]string{
				"bucket": m.DataStores[0].TrainingResults.Container,
			},
		})
	}

	if m.Framework.ImageLocation != nil {
		r.ModelDefinition.Framework.ImageLocation = &grpc_trainer_v2.ImageLocation{
			Registry:    m.Framework.ImageLocation.Registry,
			Namespace:   m.Framework.ImageLocation.Namespace,
			AccessToken: m.Framework.ImageLocation.AccessToken,
			Email:       m.Framework.ImageLocation.Email,
		}
	}

	mem, unit, err := convertMemoryFromManifest(m.Memory)
	if err != nil {
		fmt.Printf("Incorrect memory specification in manifest: %s\n", err.Error())
	}

	r.Training.Resources = &grpc_trainer_v2.ResourceRequirements{
		Gpus:       float32(m.Gpus),
		GpuType:    string(m.GpuType),
		Cpus:       float32(m.Cpus),
		Memory:     mem,
		MemoryUnit: unit,
		Learners:   m.Learners,
		// TODO add storage support
	}

	if m.EvaluationMetrics != nil {
		r.EvaluationMetrics = marshalToTrainerEvaluationMetricsSpec(m.EvaluationMetrics)
		if r.EvaluationMetrics.ImageTag != "" {
			fmt.Printf("EMExtractionSpec ImageTag: %s\n", r.EvaluationMetrics.ImageTag)
		}
	}

	return r
}

func init() {
	rootCmd.AddCommand(trainCmd)
	// -f and -l are synonyms
	trainCmd.Flags().BoolVarP(&logsFollow, "logs", "l", false,
		"if specified, follow the log")
	trainCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false,
		"if specified, follow the log")
	trainCmd.Flags().BoolVarP(&emetricsFollow, "emetrics", "e", false,
		"if specified, follow the emetrics")
	trainCmd.Flags().BoolVarP(&logsOutput, "logsOutput", "o", false,
		"if specified, logsOutput log or emetrics as TRAINING_ID.log to AISphere/dlaas-user-guide/")
	trainCmd.Flags().BoolVarP(&logsTeeOutput, "tee", "t", false,
		"if specified and logs or emetrics is also specified, write to stdout also")
	trainCmd.Flags().BoolVarP(&logsJSON, "json", "j", false,
		"if specified, logsOutput logs as json")
}
