# kubedredger

kubedredger is an example controller which manages the configuration files hierarchy for
an hypotetical application.
It is meant to be an educational tool.

## Description
This project is an educational tool which complements the golab.io 2025 session.
While the project attempts to be realistic and adopts best practices and
it is based on production ready framework and libraries, it is an example which
should never be used anywhere near production environments.

What is "kubedredger"?
The name aims to keep the sea/water/marine kubernetes theme.

**dredge**
```
 _verb: dredge; 3rd person present: dredges; past tense: dredged; past participle: dredged; gerund or present participle: dredging_

    clear the bed of (a harbour, river, or other area of water) by scooping out mud, weeds, and rubbish with a dredge.
    "the lower stretch of the river had been dredged"
    bring up or clear (something) from a river, harbour, or other area of water with a dredge.
    "mud was dredged out of the harbour"
    bring something unwelcome and forgotten or obscure to people's attention.
    "I don't understand why you had to dredge up this story"

*_noun: dredge; plural noun: dredges_

    an apparatus for bringing up objects or mud from a river or seabed by scooping or dragging.
```

This projects keeps the configuration stream clean and tidy so the application can safely navigate it.

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes cluster through [kind](https://kind.sigs.k8s.io/docs/user/quick-start/)

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/kubedredger:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Kubedredger to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/kubedredger:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
