Sometimes it makes sense to use separate Gor instance for replaying traffic and performing things like load testing, so your production machines do not spend precious resources. It is possible to configure Gor on your web machines forward traffic to Gor aggregator instance running on the separate server.

```bash
# Run on servers where you want to catch traffic. You can run it on each `web` machine.
sudo gor --input-raw :80 --output-tcp replay.local:28020

# Replay server (replay.local).
gor --input-tcp replay.local:28020 --output-http http://staging.com
```

If you have multiple replay machines you can split traffic among them using `--split-output` option: it will equally split all incoming traffic to all outputs using round robin algorithm.
```
gor --input-raw :80 --split-output --output-tcp replay1.local:28020 --output-tcp replay2.local:28020
```

[GoReplay PRO](https://goreplay.com/pro.html) support accurate recording and replaying of tcp sessions, and when `--recognize-tcp-sessions` option is passed, instead of round-robin it will use a smarter algorithm which ensures that same sessions will be sent to the same replay instance.


In case if you are planning a large load testing, you may consider use separate master instance which will control Gor slaves which actually replay traffic. For example:
```
# This command will read multiple log files, replay them on 10x speed and loop them if needed for 30 seconds, and will distributed traffic (tcp session aware) among multiple workers
gor --input-file logs_from_multiple_machines.*|1000% --input-file-loop --exit-after 30s --recognize-tcp-sessions --split-output --output-tcp worker1.local --output-tcp worker2.local:27017 --output-tcp worker3.local:27017 ...  --output-tcp workerN.local:27017

# worker 
gor --input-tcp :27017 --ouput-http load_test.target
```
