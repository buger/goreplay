### Dependencies
To start working with Gor, you need to have a web server running on your machine, and a terminal to run commands. If you are just poking around, you can quickly start the server by calling `gor file-server :8000`, this will start a simple file server of the current directory on port `8000`. 

### Installing Gor
Download the latest Gor binary from https://github.com/buger/gor/releases (we provide precompiled binaries for Windows, Linux x64 and Mac OS), or you can compile by yourself [[Compilation]].

Once the archive is downloaded and uncompressed, you can run Gor from the current directory, or you may want to copy binary to your PATH (for Linux and Mac OS it can be `/usr/local/bin`).

### Capturing web traffic
Now run this command in terminal: `sudo ./gor --input-raw :8000 --output-stdout`

This command says to listen for all network activity happening on port 8000 and log it to stdout.
If you are familiar with `tcpdump`, we are going to implement similar functionality. 

> You may notice that it uses `sudo` and asks for the password: to analyze network, Gor needs permissions which are available only to super users.
> However, it is possible to configure Gor [being run for non-root users](Running-as-non-root-user).


Make a few requests by opening `http://localhost:8000` in your browser, or just by calling curl in terminal `curl http://localhost:8000`. You should see that `gor` outputs all the HTTP requests and responses right to the terminal window where it is running. 


**Gor is not a proxy:** you do not need to put 3-rd party tool to your critical path. Instead Gor just silently analyzes the traffic of your application and does not affect it anyhow.

### Replaying

Now it's time to replay your original traffic to another environment. Let's start the same file web server but on a different port: `gor file-server :8001`. 

Instead of `--output-stdout` we will use `--output-http` and provide URL of second server: `sudo ./gor --input-raw :8000 --output-http="http://localhost:8001"`

Make few requests to first server. You should see them replicated to the second one, voila! 

### Saving requests to file and replaying them later
Sometimes it's not possible to replay requests in real time; Gor allows you to save requests to the file and replay them later. 

First use `--output-file` to save them: `sudo ./gor --input-raw :8000 --output-file=requests.gor`. This will create new file and continuously write all captured requests to it. 

Let's re-run Gor, but now to replay requests from file: `./gor --input-file requests.gor --output-http="http://localhost:8001"`. You should see all the recorded requests coming to the second server, and they will be replayed in the same order and with exactly same timing as they were recorded.

Next: [[The Basics]]

### Watch an overview:


![YOUTUBE](https://www.youtube.com/watch?v=CxuKZcMKaW4)