# FfDL CLI

User guide of gRPC CLI for Fabric for Deep Learning (FfDL) for AI Sphere.

## 1. Install FfDL gRPC CLI
### 1.1 Clone repository
```
cd $GOPATH/src/github.com/AISphere/
git clone https://github.com/AISphere/ffdl-cli.git
```
### 1.2 Add CLI path
For example, if you are using bash, you can add the path in your `.bash_profile`
```
# ffdl
export PATH="$PATH:$GOPATH/src/github.com/AISphere/ffdl-cli/bin"
```
### 1.3 Build FfDL CLI
Use glide to install all dependencies and build the CLI. (Install glide: https://glide.sh/)
```
glide install
make build
```
### 1.4 Verify CLI installed
```
➜  ~ ffdl

Description:
  ffdl is a gRPC CLI for the Fabric for Deep Learning.
 ·Users need to define the following environmental variables:
   - DLAAS_GRPC is the gRPC address that points to your AISphere cluster
   - DLAAS_USERID is a uniqe username string defined by users.

Usage:
  ffdl [command]

Available Commands:
  delete               Delete a model
  download             Download the trained model to local
  emetrics             View the ongoing training logs
  generate-completions Generate bash completions file
  halt                 Halt a training Job (not implemented)
  help                 Help about any command
  list                 List all of models
  loglines             View the ongoing training logs
  scale-test           Scale testing
  show                 Get detailed information about models
  status               Show model training status
  train                Start training a model
  version              Show CLI version and build time

Flags:
      --config string   config file (default is $HOME/.dlaas-user-guide.yaml)
  -h, --help            Help for FfDL CLI
  -t, --toggle          Help message for toggle

Use "ffdl [command] --help" for more information about a command.

```
## 2. Using FfDL gRPC CLI
### 2.1 Set environment variables
`DLAAS_GRPC` is the gRPC address. It should point to your FfDL cluster.
`DLAAS_USERID` is the ID string defined by users. *Note* we should only use letters and numbers here because special characters like `@` will return errors.
`FFDL_GRPC_CERT` is the Base64 encoded ca.crt certificate with headers as it can be found in
`ffdl-trainer/envs/dev_values.yaml`.
```
export DLAAS_GRPC="my_cluster_grpc_endpoint"
export DLAAS_USERID="my_userid"
export FFDL_GRPC_CERT="..."
```

If you're a developer, set your Kubernetes context, and call `make cli-config` to see how to set the variables.
```
$ make cli-config
# To use the FfDL gRPC CLI, set the following environment variables:
export DLAAS_USERID=user-foo  # replace with your name
export DLAAS_GRPC=xxx.xxx.xxx.xxx:pppp
```

You can check your current env variables the FfDL CLI:
```
env | grep DLAAS_
```

### 2.2 CLI commands
#### 2.2.1 Train a new model
```
➜ ffdl train --help

Start training a model

Usage:
  ffdl train MANIFEST_PATH MODEL_PATH [flags]

Flags:
  -e, --emetrics     if specified, follow the emetrics
  -h, --help         help for train
  -j, --json         if specified, logsOutput logs as json
  -l, --logs         if specified, follow the log
  -o, --logsOutput   if specified, logsOutput log or emetrics as TRAINING_ID.log to deep-learning-platform/dlaas-user-guide/
  -t, --tee          if specified and logs or emetrics is also specified, write to stdout also

Global Flags:
      --config string   config file (default is $HOME/.dlaas-user-guide.yaml)
```
Example:
```
$ ffdl train manifest.yml caffe-inc-model.zip 
➜  ffdl train manifest.yml tds-caffe-mnist-model  -l
Creating training job...
tds-caffe-mnist-model
==========
{
	"Name": "mnist-caffe-model",
	"Description": "Caffe MNIST model running on GPUs.",
	"Version": "1.0",
	"Cpus": 0,
	"Gpus": 1,
	"Learners": 0,
	"Memory": "1000MiB",
	"DataStores": [
		{
			"ID": "sl-internal-os",
			"Type": "mount_cos",
			"TrainingData": {
				"Container": "input_bucket"
			},
			"TrainingResults": {
				"Container": "output_bucket"
			},
			"Connection": {
				"auth_url": "https://s3-api.dal-us-geo.objectstorage.service.networklayer.com",
				"password": "<PASSWORD>",
				"user_name": "<USERNAME>"
			}
		}
	],
	"Framework": {
		"Name": "caffe",
		"Version": "1.0-py2",
		"Command": "caffe train -gpu all -solver lenet_solver.prototxt"
	},
	"EvaluationMetrics": {
		"Type": "regex_extractor",
		"ImageTag": "",
		"In": "$JOB_STATE_DIR/logs/training-log.txt",
		"LineLookahead": 4,
		"EventTypes": null,
		"Groups": {
			"test": {
				"Regex": "%{GLOG_STAMP:glogstamp}.*Iteration %{INT:iteration}, Testing.*\n.*Test net output .* accuracy = %{FLOAT:accuracy}\n.*Test net output .* loss = %{FLOAT:loss}",
				"Meta": {
					"Time": "$glogstamp"
				},
				"Scalars": {
					"accuracy": {
						"Type": "FLOAT",
						"Value": "$accuracy"
					},
					"lr": {
						"Type": "FLOAT",
						"Value": "$loss"
					}
				},
				"Etimes": {
					"glog": {
						"Type": "STRING",
						"Value": "$glogstamp"
					},
					"iteration": {
						"Type": "INT",
						"Value": "$iteration"
					}
				}
			},
			"train": {
				"Regex": "%{GLOG_STAMP:glogstamp}.*solver.cpp:.*Iteration %{INT:iteration} \\(.*\n.*solver.cpp.*Train .*loss = %{FLOAT:loss}.*\n.*.lr = %{FLOAT:lr}",
				"Meta": {
					"Time": "$glogstamp"
				},
				"Scalars": {
					"learning-rate": {
						"Type": "FLOAT",
						"Value": "$lr"
					},
					"loss": {
						"Type": "FLOAT",
						"Value": "$loss"
					}
				},
				"Etimes": {
					"glog": {
						"Type": "STRING",
						"Value": "$glogstamp"
					},
					"iteration": {
						"Type": "INT",
						"Value": "$iteration"
					}
				}
			}
		}
	}
}

==========
EMExtractionSpec ImageTag:
Training job created. Training ID is training-f3T_WHqkR
```
#### 2.2.2 List all models
```
➜  ffdl list --help
List all of models

Usage:
  ffdl list [flags]

Flags:
  -h, --help    help for list
  -s, --short   if specified, just print the training id

Global Flags:
      --config string   config file (default is $HOME/.dlaas-user-guide.yaml)
```
Example:
```
$ ffdl list
Connected to trainer
Getting all training Jobs...
ID                 NAME                             FRAMEWORKS STATUS                                                      START                                                       COMPLETED
training-dx5UFS3kg mnist-pytorch-model              pytorch    Learner process terminated with an error (exit code: '1')   2018-02-12 08:53:30.350848079 +0000 UTC m=+304353.618638136 2018-02-12 08:55:19.474942438 +0000 UTC m=+304462.742733749
training-tcLs5S3zg mnist-pytorch-model              pytorch    Learner process terminated with an error (exit code: '1')   2018-02-12 09:10:35.926438881 +0000 UTC m=+305380.294991370 2018-02-12 09:12:23.0261865 +0000 UTC m=+305481.646364310
training-by_KZH3kg nlc-model                        torch      Learner process terminated with an error (exit code: '1')   2018-02-12 15:46:02.575113263 +0000 UTC m=+328012.252803352 2018-02-12 15:47:05.07467316 +0000 UTC m=+328074.752363270
training-mDAAWN3zR mnist-caffe-model                caffe                                                                  2018-02-12 15:48:36.236463299 +0000 UTC m=+329260.605014841 2018-02-12 15:49:54.632651625 +0000 UTC m=+329339.001203160
training-f3T_WHqkR mnist-caffe-model                caffe                                                                  2018-02-12 15:54:04.126156866 +0000 UTC m=+329582.746334558 2018-02-12 15:55:27.430932401 +0000 UTC m=+329666.051110288
training-tT7eZHqkR mnist-caffe-model                caffe                                                                  2018-02-12 15:56:37.065814731 +0000 UTC m=+329740.333605278 2018-02-12 15:57:23.209668012 +0000 UTC m=+329787.578219621
```
#### 2.2.3 Show model details
```
➜  ffdl show --help
Get detailed information about models

Usage:
  ffdl show TRAINING_ID [flags]

Flags:
  -h, --help   help for show

Global Flags:
      --config string   config file (default is $HOME/.dlaas-user-guide.yaml)
```
Example:
```
$ ffdl show training-rElol5Xzg
Connected to trainer
Querying model with ID: training-mDAAWN3zR ...
{
  "training_id": "training-rElol5Xzg",
  "user_id": "e2e-test-user",
  "model_definition": {
    "name": "caffe-inc-model",
    "description": "Caffe incremental workload with pretrained model.",
    "location": "ffdl-models/training-rElol5Xzg.zip",
    "framework": {
      "name": "caffe",
      "version": "1.0-py2"
    }
  },
  "training": {
    "command": "caffe train -solver ffdl_inc_solver.prototxt -gpu all -weights ./ffdl_inc_pretrained_resNet20.caffemodel",
    "resources": {
      "cpus": 8,
      "gpus": 2,
      "memory": 12,
      "memory_unit": 3,
      "schedpolicy": "dense"
    },
  },
  ...
  "datastores": [
    {
      "id": "sl-internal-os-input",
      "type": "softlayer_objectstore",
      "fields": {
        "bucket": "caffe_inc_data_slim"
      }
    },
	...
  ],
  "job_id": "025eccc8-1e62-4aa0-6d73-487c94cfc621"
}
```
#### 2.2.4 Monitor a model
```
➜  ffdl status --help
Show model training status

Usage:
  ffdl status TRAINING_ID [flags]

Flags:
  -h, --help   help for status

Global Flags:
      --config string   config file (default is $HOME/.dlaas-user-guide.yaml)
```
Example:
```
➜  ffdl status training-mDAAWN3zR
Getting training status ID 'training-mDAAWN3zR' ...
Training Status is Status_COMPLETED
```
#### 2.2.5 Download training log
```
View the ongoing training logs

Usage:
  ffdl loglines TRAINING_ID [flags]

Flags:
  -f, --follow           if specified, follow the log
  -h, --help             help for loglines
  -j, --json             if specified, logsOutput logs as json
  -o, --logsOutput       if specified, logsOutput log as TRAINING_ID.log to deep-learning-platform/dlaas-user-guide/
  -s, --pagesize int32   Number of lines to deliver (default 10)
  -p, --pos int          If positive, line number from start, if negative, line position from end (default 1)
  -i, --subid string     if specified use subid as qualifier
  -t, --tee              if specified and logsOutput is also specified, write to stdout also

Global Flags:
      --config string   config file (default is $HOME/.dlaas-user-guide.yaml)
```
Example:
```
➜  ffdl loglines training-mDAAWN3zR
1 1518450564892917558 Training with training/test data and model at:
2 1518450564907692550   DATA_DIR: /mnt/input_bucket
3 1518450564915767925   MODEL_DIR: /job/model-code
4 1518450564921847654   TRAINING_JOB:
5 1518450564926775024   TRAINING_COMMAND: caffe train -gpu all -solver lenet_solver.prototxt
6 1518450564933468337 CAFFE_ROOT=/opt/caffe
7 1518450564984233573 CAFFE_VERSION=1.0
8 1518450564989627939 CSF_HELLO_WORLD_PORT=tcp://172.21.23.185:80
9 1518450564994879293 CSF_HELLO_WORLD_PORT_443_TCP=tcp://172.21.23.185:443
10 1518450565000143000 CSF_HELLO_WORLD_PORT_443_TCP_ADDR=172.21.23.185
```
#### 2.2.6 Download trained model
```
➜  ffdl download --help
Download the trained model to local

Usage:
  ffdl download TRAINING_ID (--definition|--trainedmodel) [flags]

Flags:
  -d, --definition     download the model definition
  -h, --help           help for download
  -m, --trainedmodel   download the trained model

Global Flags:
      --config string   config file (default is $HOME/.dlaas-user-guide.yaml)
```
```
➜  ffdl download training-mDAAWN3zR
Please add '-d' or '-m' flag. More detail see 'ffdl download --help'
➜  ffdl download training-mDAAWN3zR -d
Downloading model definition 'training-mDAAWN3zR' ...
Model definition training-mDAAWN3zR is downloaded to training-mDAAWN3zR_definition.zip successfully
➜  ffdl download training-mDAAWN3zR -m
Downloading trained model 'training-mDAAWN3zR' ...
Trained model training-mDAAWN3zR is downloaded to training-mDAAWN3zR_trainedmodel.zip successfully
```
#### 2.2.7 Delete model
```
Delete a model

Usage:
  ffdl delete TRAINING_ID [flags]

Flags:
  -h, --help   help for delete
```
```
$ ffdl delete training-rElol5Xzg
Connected to trainer
Deleting model 'training-rElol5Xzg' ...
Training model training-rElol5Xzg deleted
```
#### 2.2.8 Halt a training
```
➜  ~ ffdl halt -h
Halt a training Job

Usage:
  ffdl halt TRAINING_ID [flags]

Flags:
  -h, --help   help for halt

Global Flags:
      --config string   config file (default is $HOME/.dlaas-user-guide.yaml)
```
#### 2.2.9 Version info
```
$ ffdl version
CLI version: DLaaS_CLI 1.0
Build Time : 20190101
```

# Acknowledgments
Ursula Zhou<Ursula.Zhou@ibm.com> (initial author)
