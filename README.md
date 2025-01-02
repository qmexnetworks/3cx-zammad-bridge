> [!CAUTION]
> So far its not working with 3CX V20, 3CX announced a new API for Update 2 as soon as the new API is out we can make the bridge work again, as 3CX V20 is not feature complete yet, you should stick with V18  
> UPDATE June 2024: We already did some investigations on the new API and this new API doesnt give us the tools to make this extension ready for V20. The only way will be the call control API (https://www.3cx.com/blog/releases/v20-call-control-api/) so far we have only .NET but a web version is announced which will be necerssary.
> UPDATE 30. July 2024: So far the new call control Web API was NOT released with V20 U2

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
    country_prefix: 49 # numeric; optional; The country dialing prefix to remove from the numbers

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

Example systemd service:

```unit file (systemd)
[Unit]
Description=3cx-zammad-bridge
After=network.target

# If running on the same machine as 3CX, you might want to wait for it to start
PartOf=3CXGatewayService.service

[Service]
User=zammad-bridge
Group=zammad-bridge
ExecStart=/usr/local/bin/zammadbridge
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

## Help
```
3cx-zammad-bridge is a bridge that listens on 3cx to forward information to zammad

Usage:
  zammadbridge [flags]

Flags:
  -h, --help                help for zammadbridge
  -f, --log-format string   log format: "json" or "plain" (default "json")
      --trace               trace output, super verbose
  -v, --verbose             verbose output
```

## Development

You can build the binary by running `make build`

Theoretically, this should also run on Windows. You can compile it yourself and
report possible issues. 
