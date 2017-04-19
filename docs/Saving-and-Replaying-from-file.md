You can save requests to file, and replay them later. While replaying it will preserve the original time differences between requests. If you apply [percentage based limiting](Rate Limiting) timing between requests will be reduced or increased appropriately: this approach opens possibilities like load testing, see below.

```bash
# write to file
gor --input-raw :80 --output-file requests.log

# read from file
gor --input-file requests.gor --output-http "http://staging.com"
```

By default Gor writes files in chunks. This configurable using `--output-file-append` option: the flushed chunk is appended to existence file or not. The default is **false**. By default, `--output-file` flushes each chunk to a different path.

```bash
gor ... --output-file %Y%m%d.log
# append false
20140608_0.log
20140608_1.log
20140609_0.log
20140609_1.log
```

This makes parallel file processing easy. But if you want to disable this behavior, you can disable it by adding `--output-file-append` option:

```bash
gor ... --output-file %Y%m%d.log --output-file-append
# append true
20140608.log
20140609.log
```

If you run gor multiple times, and it finds existing files, it will continue from last known index.

### Chunk size

You can set chunk limits using `--output-file-size-limit` and `--output-file-queue-limit` options.
The length of the chunk queue and the size of each chunk, respectively. The default values are 256 and 32mb, respectively. The suffixes “k” (KB), “m” (MB), and “g” (GB) can be used for `output-file-size-limit`.
If you want to have only size constraint, you can set `--output-file-queue-limit` to 0, and vice versa.

```bash
gor --input-raw :80 --output-file %Y-%m-%d.gz --output-file-size-limit 256m --output-file-queue-limit 0
```

### Using date variables in file names
For example, you can tell to create new file each hour: `--output-file /mnt/logs/requests-%Y-%m-%d-%H.log`
It will create new file for each hour: requests-2016-06-01-12.log, requests-2016-06-01-13.log, ...

The time format used as part of the file name. The following characters are replaced with actual values when the file is created:

* `%Y`: year including the century (at least 4 digits)
* `%m`: month of the year (01..12)
* `%d`: Day of the month (01..31)
* `%H`: Hour of the day, 24-hour clock (00..23)
* `%M`: Minute of the hour (00..59)
* `%S`: Second of the minute (00..60)

The default format is `%Y%m%d%H`, which creates one file per hour.


### GZIP compression
To read or write GZIP compressed files ensure that file extension ends with ".gz": `--output-file log.gz`

### Replaying from multiple files

`--input-file` accepts file pattern, for example: `--input-file logs-2016-05-*`: it will replay all the files, sorting them in lexicographical order.

### Buffered file output
Gor has memory buffer when it writes to file, and continuously flush changes to the file. Flushing to file happens if the buffer is filled, forced flush every 1 second, or if Gor is closed. You can change it using `--output-file-flush-interval` option. It most cases it should not be touched.

### File format
HTTP requests stored as it is, plain text: headers and bodies. Requests separated by `\n🐵🙈🙉\n` line (using such sequence for uniqueness and fun). Before each request goes single line with meta information containing payload type (1 - request, 2 - response, 3 - replayed response), unique request ID (request and response have the same) and timestamp when request was made. An example of 2 requests:

```
1 d7123dasd913jfd21312dasdhas31 127345969\n
GET / HTTP/1.1\r\n
\r\n
\n
🐵🙈🙉
\n
POST /upload HTTP/1.1\r\n
Content-Length: 7\r\n
Host: www.w3.org\r\n
\r\n
a=1&b=2
```
Note that technically \r and \n symbols are invisible, and indicate new lines. I made them visible in example just to show how it looks on byte level.

Making it text friendly allows writing simple parsers and use console tools like `grep` to do an analysis. You can even edit them manually, but be sure that your file editor does not change line endings.

## Performance testing

Currently, this functionality supported only by `input-file` and only when using percentage based limiter. Unlike default limiter for `input-file` instead of dropping requests it will slowdown or speedup request emitting. Note that **limiter is applied to input**:

```
# Replay from file on 2x speed 
gor --input-file "requests.gor|200%" --output-http "staging.com"
```

Use `--stats --output-http-stats` to see latency stats.

### Looping files for replaying indefinitely
You can loop the same set of files, so when the last one replays all the requests, it will not stop, and will start from first one again. Having the only small amount of requests you can do extensive performance testing.
Pass `--input-file-loop` to make it work. 

***
You may also read about [[Capturing and replaying traffic]] and [[Rate limiting]]