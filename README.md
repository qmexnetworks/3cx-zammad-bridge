> [!CAUTION]
> Support for v20 and above is experimental and may not work as expected. Please report any issues you encounter.
> Currently, only monitoring of extensions is supported. Monitoring of groups is not supported due to limitations of the 3CX permissions model.

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

The first (found) configuration file will be used. Also refer to the `config.yaml.dist` file.

For 3CX versions 20 and above, it's important that you create a client ID and secret in the 3CX web interface. 
You have to add all extensions that you want to monitor to the Call Control API permissions in the 3CX web interface for
the client ID you create.
Note that monitoring groups this way is not supported (due to limitations of the 3CX permissions model). 
You have to add all extensions manually.

Example configuration:

```yaml
Bridge:
  poll_interval: 0.5 # decimal; The number of seconds to wait in between polling 3CX for calls

3CX:
    # For versions below v20, define these two:
    user: "the username of a 3CX admin account"
    pass: "the password of a 3CX admin account"
    group: "the name of the 3CX group that should be monitored, for example Support"
    # For versions v20 and above, define these two:
    client_id: "the client ID you created in 'Admin' -> 'Integrations' -> 'API'"
    client_secret: "the secret that was shown once"
    # Always define these:
    host: "the URL of your 3CX server, including https://"
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

# One might want to wait for the 3CXGatewayService to be up and running
# before starting this service, but during updates the 3CGatewayService
# is *stopped* and later started. This results in this 3cx-zammad-bridge
# to be stopped but never started again.
#PartOf=3CXGatewayService.service

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
  -c, --config string       custom config file path (default "/etc/3cx-zammad-bridge/config.yaml")
  -h, --help                help for zammadbridge
  -f, --log-format string   log format: "json" or "plain" (default "json")
      --trace               trace output, super verbose
  -v, --verbose             verbose output
```

## Development

You can build the binary by running `make build`

Theoretically, this should also run on Windows. You can compile it yourself and
report possible issues. 

## Support

Premium support is available at [Q-MEX Networks GmbH](https://www.qmex.net)
