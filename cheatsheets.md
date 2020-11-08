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
  
 ```