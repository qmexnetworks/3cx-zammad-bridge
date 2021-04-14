# 3cx-zammad-bridge

Monitors calls in 3CX and communicates this to Zammad accordingly.

## Requirements

 - Linux x86_64

## Installation

- Download the latest release binary from [releases](https://github.com/qmexnetworks/3cx-zammad-bridge/releases).
    - Copy the binary into `/usr/local/bin`
- `chmod +x zammadbridge`

## Configuration

All configuration is done through the `config.yaml` file, that may appear in these locations:

- `/etc/3cx-zammad-bridge/config.yaml`
- `/opt/3cx-zammad-bridge/config.yaml`
- `config.yaml`  (within the working directory of this 3cx bridge process) 

The first (found) configuration file will be used. Also refer to the `config.yaml.dist` file
   
```yaml
Bridge:
  poll_interval: 0.5 # decimal; The number of seconds to wait in between polling 3CX for calls

3CX:
    user: "the username of a 3CX admin account"
    pass: "the password of a 3CX admin account"
    host: "the URL of your 3CX server, including https://"
    group: "the name of the 3CX group that should be monitored, for example Support"
    extension_digits: 3 # numeric; How many digits the internal extensions have 
    trunk_digits: 5 # numeric; How many digits the numbers in the trunk have
    queue_extension: 816 # numeric; The number of the queue that the bridge should also listen to

Zammad:
    endpoint: https://zammad.example.com/api/v1/cti/secret # The URL of your Zammad server, including the secret in the URL
    log_missed_queue_calls: true # boolean; Whether or not you want to log missed calls to your queue
```

## Running
 
Run the release binary to run the daemon. 

Example supervisord config:

```ini
[program:3cx-zammad-bridge]
command = /usr/local/bin/zammadbridge
autostart = true
autorestart = true
startretries = 10
stderr_logfile = /var/log/3cx-zammad-bridge.err.log
stdout_logfile = /var/log/3cx-zammad-bridge.out.log

# Optionally specify a user
user = zammad-bridge
```

## Development

You can build the binary by running `make build`

Theoretically, this should also run on Windows. You can compile it yourself and
report possible issues. 
