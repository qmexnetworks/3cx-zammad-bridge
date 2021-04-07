# 3cx-zammad-bridge

Monitors calls in 3CX and communicates this to Zammad accordingly.

## Requirements

- Python 3.9+

## Installation

```shell
# Recommended: use virtualenv
python -m venv venv
source venv/bin/activate
pip install pipenv
pipenv install
# Run the service
python src
```
 
## Configuration

All configuration is done through the `config.yaml` file, that may appear in these locations:

- `/etc/3cx-zammad-bridge/config.yaml`
- `/opt/3cx-zammad-bridge/config.yaml`
- `config.yaml`  (within the working directory of this 3cx bridge process) 

The first (found) configuration file will be used. Also refer to the `config.yaml.dist` file
   
```yaml
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
 
Run the `src/__main__.py` script to run the daemon. 

Example supervisord config:

```ini
[program:3cx-zammad-bridge]
directory=/opt/3cx-zammad-bridge/
command=/opt/3cx-zammad-bridge/venv/bin/python3.9 src
user=zammad-bridge
autostart=true
autorestart=true
startretries=10
stderr_logfile=/var/log/3cx-zammad-bridge.err.log
stdout_logfile=/var/log/3cx-zammad-bridge.out.log
```

