 ```bash

##########################
## Steps create a Resource
##########################
# 1. Creates the CRD Definition
  /artifacts/{Resource}/crd.yaml
# 2 .Creates the Resource API
  # 2.1 Creates folder
    /pkg/apis/{Resource}/{Version}/
  # 2.2 Creates register.go
    /pkg/apis/{Resource}/register.go
    /pkg/apis/{Resource}/doc.go
    /pkg/apis/{Resource}/types.go
# 3. Download the generator of boilerplate code
  go mod vendor  
# 4 .Creates the Boilerplate code with the script
  ./hack/update-codegen.sh

##########################
## Cheat Sheet
##########################
$ go mod init main.go 
$ go run main.go

##########################
## BUILD & RUN (locally)
##########################
$ kubectl apply -f artifacts/podbuggertool/crd.yaml  # Install the CRD at Kubernetes first
#$  kubectl get crd
#$  kubectl api-resources | grep podbuggertool # check the CRD installed
$ go build  .  # Build the controller (executable object)
$ ./podbuggertool -kubeconfig ~/.kube/config  -logtostderr=true # Run it
# Another shell, install a podbuggertool
$ kubectl apply -f artifacts/podbuggertool/podbuggertool.yaml
# Watch the pods
$ kubectl get pods -w
# Using our CRD API Resource
$ kubectl get podbuggertool
 

########################################
## Packaging (Docker) & Install (K8s)
########################################
docker build . -t ualter/podbuggertool
docker push ualter/podbuggertool
## Deploy the Operator it at Kubernetes
# 1. First we install the CRD (Custom Resource Definition), the Resource API
k apply -f /artifacts/podbuggertool/crd.yaml       
# 2. Install the permissions needed, in order that the operator can do its job (ServiceAccount: default at Namespace: develop)
k apply -f deploy-podbuggertool-permissions.yaml
# 3. Install the Operator itself (The controller it is a Golang application)
k apply -f deploy-podbuggertool-controller.yaml

########################################
## Using the Operator (as user of K8s)
########################################
## Creating PodBuggerTool instances
k apply -f ./artifacts/podbuggertool/podbuggertool-teachstore.yaml

```