import ssl

import requests

from bridge_helpers.ssl import SSLAdapter
from config.config import BridgeConfig


class Api3CX:
    is_authenticated = False

    def __init__(self, config: BridgeConfig):
        self.session = requests.Session()
        self.session.mount('https://', SSLAdapter(ssl.PROTOCOL_TLSv1_2))
        self.config = config
        self.extensions = []

        self.authenticate()

    def fetch_group_members(self, group):
        """
         Get members by group to only monitor calls of this group

        :param group: str
        :return: list[int]
        """
        nI = 0
        extensions = []
        r = self.session.get(self.config.api3CX_host + '/api/GroupList')
        data = r.json()

        group_id = None
        for row in data['list']:
            # Get Group ID for Right Group
            if str(row['Name']) == group:
                group_id = int(row['Id'])
                group_extension_count = int(row['ExtensionsCount'])

        if group_id is None:
            raise RuntimeError('Group in 3CX not found: ' + group)
        # Get all group members
        req_data = {
            "Id": group_id
        }
        r = self.session.post(self.config.api3CX_host + '/api/GroupList/set', json=req_data)
        data = r.json()
        # Save temp. group ID given by 3CX Request for paging
        req_group_id = int(data['Id'])
        # If there are more than 10, use paging
        while group_extension_count > nI:
            req_data = {
                "Path": {
                    "ObjectId": req_group_id,
                    "PropertyPath": [{
                        "Name": "Members"
                    }],
                },
                "PropertyValue": {
                    "State": {
                        "Start": nI,
                        "SortBy": "null",
                        "Reverse": "false",
                        "Search": ""
                    }
                }
            }
            r = self.session.post(self.config.api3CX_host + '/api/edit/update', json=req_data)
            data = r.json()
            for row in data[0]['Item']['Members']['selected']:
                nI += 1
                extensions.append(int(row['Number']['_value']))
        print("Imported " + str(nI) + " extensions: ")
        for extension in extensions:
            print('- ' + str(extension))

        ## Add Queue extension to Extensions
        extensions.append(self.config.api3CX_queue_extension)

        return extensions

    def authenticate(self):
        auth_data = {
            "username": self.config.api3CX_user,
            "password": self.config.api3CX_pass
        }

        resp = self.session.post(self.config.api3CX_host + '/api/login', json=auth_data)
        if resp.status_code == 200 and str(resp.text) == 'AuthSuccess':
            self.is_authenticated = True
            self.extensions = self.fetch_group_members(self.config.api3CX_group)
        else:
            raise RuntimeError('Unable to authenticate with 3CX: ' + resp.text)

    def fetch_active_calls(self) -> dict:
        resp = self.session.get(self.config.api3CX_host + '/api/activeCalls')
        if resp.status_code != 200:
            # Try reauth
            print("Reauthenticating...")
            self.authenticate()

            return self.fetch_active_calls()
            # TODO protect against infinite loop

        return resp.json()
