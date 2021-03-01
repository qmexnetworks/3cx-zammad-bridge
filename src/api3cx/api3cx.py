from datetime import datetime, timedelta
import time
import logging

import requests

from config.config import BridgeConfig


class Api3CX:
    down_since = None
    is_authenticated: bool = False
    config: BridgeConfig
    extensions: list[int]

    def __init__(self, config: BridgeConfig):
        self.session = requests.Session()
        self.config = config

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
        logging.info("Imported " + str(nI) + " extensions: ")
        for extension in extensions:
            logging.info('- ' + str(extension))

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
            self.down_since = None
            self.is_authenticated = True
            self.extensions = self.fetch_group_members(self.config.api3CX_group)
        else:
            # Start logging the point it went down
            if self.down_since is None:
                self.down_since = datetime.now()

            # If it has been down for two minutes, exit
            if datetime.now() - timedelta(minutes=2) > self.down_since:
                raise RuntimeError('Unable to authenticate with 3CX for two minutes: ' + resp.text)

            # Reauthenticate after 5 seconds
            logging.warn("Unable to authenticate with 3CX (possibly offline?) - HTTP Status %s. Retrying in 5 sec..." % resp.status_code)
            time.sleep(5)
            return self.authenticate()

    def fetch_active_calls(self) -> dict:
        resp = self.session.get(self.config.api3CX_host + '/api/activeCalls')
        if resp.status_code != 200:
            # Try reauth
            logging.info("Reauthenticating...")
            self.authenticate()

            return self.fetch_active_calls()
            # TODO protect against infinite loop

        return resp.json()
