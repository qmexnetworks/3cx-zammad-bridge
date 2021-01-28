import yaml


class BridgeConfig:
    def __init__(self, *paths):
        for path in paths:
            try:
                with open(path, 'r') as stream:
                    try:
                        self.data = yaml.safe_load(stream)
                        self.api3CX_host = self.data['3CX']['host']
                        if not self.api3CX_host.startswith('http'):
                            raise RuntimeError('3CX Host needs to be a full URL with protocol, e.g. https://3cx.example.com')

                        self.api3CX_user = self.data['3CX']['user']
                        self.api3CX_pass = self.data['3CX']['pass']
                        self.api3CX_group = self.data['3CX']['group']
                        self.api3CX_extension_digits = int(self.data['3CX']['extension_digits'])
                        self.api3CX_trunk_digits = int(self.data['3CX']['trunk_digits'])
                        self.api3CX_group = self.data['3CX']['group']
                        self.api3CX_queue_extension = int(self.data['3CX']['queue_extension'])
                        self.zammad_endpoint = self.data['Zammad']['endpoint']
                        self.zammad_log_missed_queue_calls = bool(self.data['Zammad']['log_missed_queue_calls'])

                        return
                    except yaml.YAMLError as exc:
                        print(exc)
            except FileNotFoundError:
                continue  # will try other file in loop

        raise EnvironmentError('Unable to find any config file')
