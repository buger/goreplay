gor & elasticsearch
===================

Prerequisites
-------------

- elasticsearch
- kibana (Get it here: http://www.elasticsearch.org/overview/kibana/)
- gor


elasticsearch
-------------

The default elasticsearch configuration is just fine for most workloads. You won't need clustering, sharding or something like that.

In this example we're installing it on our gor replay server which gives us the elasticsearch listener on _http://localhost:9200_

(Support es6.x)


kibana
------

Kibana (elasticsearch analytics web-ui) is just as simple. 
Download it, extract it and serve it via a simple webserver.
(Could be nginx or apache)

You could also use a shell, ```cd``` into the kibana directory and start a little quick and dirty python webserver with:

```
python -m SimpleHTTPServer 8000
```

In this example we're also choosing the gor replay server as our kibana host. If you choose a different server you'll have to point kibana to your elasticsearch host.


gor
---

Start your gor replay server with elasticsearch option:

```
./gor --input-raw :8000 --output-http http://staging.com  --output-http-elasticsearch localhost:9200/gor
```

Or your can start gor with docker:

```
cd ./examples/gorRunWithDocker
docker-compose up -d
```

What's more, your can start kibana and es with docker:

<img src="https://github.com/ShaoNianyr/goreplay/blob/go-es-6x/examples/gorRunWithDocker/pictures/docker_elk.png">

(You don't have to create the index upfront. That will be done for you automatically and update everyday, like gor-2020-01-01.)

Here is an example of gor links kibana-es-6x:

<img src="https://github.com/ShaoNianyr/goreplay/blob/go-es-6x/examples/gorRunWithDocker/pictures/kibana-es-6x.png">


Troubleshooting
---------------

The replay process may complain about __too many open files__.
That's because your typical linux shell has a small open files soft limit at 1024.
You can easily raise that when you do this before starting your _gor replay_ process:

```
ulimit -n 64000
```

Please be aware, this is not a permanent setting. It's just valid for the following jobs you start from that shell.

We reached the 1024 limit in our tests with a ubuntu box replaying about 9000 requests per minute. (We had very slow responses there, should be way more with fast responses)
