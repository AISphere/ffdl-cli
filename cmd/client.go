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
	"github.com/AISphere/ffdl-trainer/trainer/grpc_trainer_v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"crypto/x509"
	"crypto/tls"
)

var (
	dlaasGrpcClient TrainerClient //Only initiate if we are using gRPC (initiate via trainModelGrpc())
	defaultBackoff  = backoff.NewExponentialBackOff()
	gRPCpath        = os.Getenv("DLAAS_GRPC")
)

// TrainerClient represents the client interface to the FfDL Trainer service
type TrainerClient interface {
	Client() grpc_trainer_v2.TrainerClient
	Close() error
}

type trainerClient struct {
	client grpc_trainer_v2.TrainerClient
	conn   *grpc.ClientConn
}

func (c *trainerClient) Client() grpc_trainer_v2.TrainerClient {
	return c.client
}

func (c *trainerClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ca.crt
var caCertB = []byte(`-----BEGIN CERTIFICATE-----
...
-----END CERTIFICATE-----`)

func createClientDialOpts() ([]grpc.DialOption, error) {
	var opts []grpc.DialOption

	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(caCertB) {
		return nil, fmt.Errorf("credentials: failed to append certificates")
	}
	newCredentials := credentials.NewTLS(&tls.Config{ServerName: "dlaas.ibm.com", RootCAs: cp})

	//creds, err := credentials.NewClientTLSFromFile("ca.crt", "dlaas.ibm.com")
	opts = []grpc.DialOption{grpc.WithTransportCredentials(newCredentials), grpc.WithBlock()}
	return opts, nil
}

// NewTrainerWithAddress create a new load-balanced client to talk to the Trainer service. If the dns_server config option is set to 'disabled', it will default to the pre-defined LocalPort of the service.
func NewTrainerWithAddress(addr string) (TrainerClient, error) {
	dialOpts, err := createClientDialOpts()
	if err != nil {
		fmt.Printf("Create dial options failed: %v\n", err)
		return nil, err
	}

	d := time.Now().Add(1 * time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()

	//fmt.Printf("Connecting to trainer service: %s\n", addr)
	conn, err := grpc.DialContext(ctx, addr, dialOpts...)
	if err != nil {
		fmt.Printf("Could not connect to trainer service: %v", err)
		return nil, err
	}

	return &trainerClient{
		conn:   conn,
		client: grpc_trainer_v2.NewTrainerClient(conn),
	}, nil
}

// NewDlaaSTrainerGrpcClient creates a connection to the DLaaS Trainer service
func NewDlaaSTrainerGrpcClient(dlaasHost string) TrainerClient {
	//fmt.Printf("Calling NewTrainerWithAddress: %s\n", dlaasHost)
	cli, err := NewTrainerWithAddress(dlaasHost)
	if err != nil {
		fmt.Println("Got error while connecting to '" + dlaasHost + "'")
		// Probably best to get out of Dodge at this point
		os.Exit(-1)
	} else {
		//fmt.Println("Connected to trainer")
	}
	return cli
}

func initTrainerClient() bool {
	if dlaasGrpcClient == nil {
		if gRPCpath == "" {
			fmt.Println("`DLAAS_GRPC` is not set")
			return false
		}
		dlaasGrpcClient = NewDlaaSTrainerGrpcClient(gRPCpath)
		if dlaasGrpcClient == nil {
			return false
		}
	}
	return true
}
