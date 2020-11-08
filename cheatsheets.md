 ```bash

# Steps create a Resource
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


  # Cheat Sheet
$ go mod init main.go 
$ go run main.go



# BUILD & RUN
$ kubectl apply -f artifacts/daemonstool/crd.yaml  # Install the CRD at Kubernetes first
#$  kubectl get crd
#$  kubectl api-resources | grep daemonstools # check the CRD installed
$ go build -o pbtctrl .  # Build the controller (executable object)
$ ./ctrl -kubeconfig ~/.kube/config  -logtostderr=true # Run it
# Another shell, install a Daemonstool
$ kubectl apply -f artifacts/daemonstool/daemonstool-example.yaml
# Watch the pods
$ kubectl get pods -w
# Using our CRD API Resource
$ kubectl get daemonstool
 

```