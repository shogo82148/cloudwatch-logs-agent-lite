# cloudwatch-logs-agent-lite
Lightweight log forwarder with CloudWatch

## SYNOPSIS

Read logs from STDIN:

```
$ echo Hello | cloudwatch-logs-agent-lite -log-group-name /test
$ aws logs tail
2022-10-29T08:34:07.441000+00:00 ichinoseshougonoMacBook-Pro.local Hello
```

Read logs from the log file `access.log`:

```
$ cloudwatch-logs-agent-lite -log-group-name /test access.log &
$ echo Hello >> access.log
$ aws logs tail
2022-10-29T08:34:07.441000+00:00 ichinoseshougonoMacBook-Pro.local Hello
```

## USAGE

```
Usage of cloudwatch-logs-agent-lite:
  -flush-interval duration
    	interval to flush the logs (default 5s)
  -flush-timeout duration
    	timeout to flush the logs (default 3m0s)
  -log-group-name string
    	log group name
  -log-level string
    	minimum log level. Possible values are: debug, info, warn, error (default "info")
  -log-retention-days int
    	Specifies the number of days you want to retain log events in the specified log group. Possible values are: 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653
  -log-stream-name string
    	log stream name
  -region string
    	aws region
  -version
    	show the version
```
