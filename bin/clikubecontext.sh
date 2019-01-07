#!/bin/bash

#--------------------------------------------------------------------------#
#                                                                          #
# Copyright 2019 IBM Corporation                                           #
#                                                                          #
# Licensed under the Apache License, Version 2.0 (the "License");          #
# you may not use this file except in compliance with the License.         #
# You may obtain a copy of the License at                                  #
#                                                                          #
# http://www.apache.org/licenses/LICENSE-2.0                               #
#                                                                          #
# Unless required by applicable law or agreed to in writing, software      #
# distributed under the License is distributed on an "AS IS" BASIS,        #
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. #
# See the License for the specific language governing permissions and      #
# limitations under the License.                                           #
#--------------------------------------------------------------------------#


# Print selected information from a Kubernetes context.
# Be sure to set the DLAAS_KUBE_CONTEXT to override the current context (i.e., kubectl config current-context)

DLAAS_KUBE_CONTEXT=${DLAAS_KUBE_CONTEXT:-$(kubectl config current-context)}

function printTrainerUrl() {
  host=$(kubectl --context="$DLAAS_KUBE_CONTEXT" get nodes -o jsonpath='{.items[0].status.addresses[0].address}')
  port=$(kubectl --context="$DLAAS_KUBE_CONTEXT" get svc/dlaas-trainer-v2 -o jsonpath='{.spec.ports[0].nodePort}' 2> /dev/null)
  if [[ -z "$host" || -z "$port" ]]; then
    # Assume run-services-local scenario.
    url="localhost:30005"
  else
    url="$host:$port"
  fi
  echo $url
}

# Parse command line args and dispatch command.
case $1 in
trainer-url)
  printTrainerUrl
  ;;
*)
  echo "Usage: clikubecontext.sh trainer-url"
  exit 1
  ;;
esac
