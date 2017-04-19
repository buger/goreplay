## STEP 1: Install Docker
For local development we recommend to use Docker.

If you don’t have it you can read how to install it here:
https://docs.docker.com/engine/getstarted/step_one/#step-3-verify-your-installation

## STEP 2: Download repository

`git clone git@github.com:buger/goreplay.git`


## STEP 3: Setup container

```
cd ./goreplay
make build

```

## Testing
To run tests execute next command:

```
make test
```

You can copy the command that is produced and modify it. For example, if you need to run one test copy the command and add `-run TestName`, e.g.:

```
docker run -v `pwd`:/go/src/github.com/buger/gor/ -p 0.0.0.0:8000:8000 -t -i gor:go go test ./. -run TestEmitterFiltered -timeout 60s -ldflags "-X main.VERSION=DEV-1482398347 -extldflags \"-static\""   -v
```


## Building
To get a binary file run 

```
make release-bin
```
