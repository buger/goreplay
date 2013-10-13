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
./gor replay -f <your-dev-system-url> -ip <your_replay_listener_ip> -p <your_replay_listener_port> -es <elasticsearch_host>:<elasticsearch_port>/<elasticsearch_index>
```

In our example this would be:

```
./gor replay -f <your-dev-system-url> -ip <your_replay_listener_ip> -p 28020 -es localhost:9200/gor
```

(You don't have to create the index upfront. That will be done for you automatically)

Now start your gor listen process as usual:

```
sudo gor listen -p 80 -r replay.server.local:28020
```

Now visit your kibana url, load the predefined dashboard from the gist https://gist.github.com/gottwald/b2c875037f24719a9616 and watch the data rush in.


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
