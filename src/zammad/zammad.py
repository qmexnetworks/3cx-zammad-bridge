import logging

import requests

from zammad.call import CallZammad


class Zammad:
    endpoint: str

    def __init__(self, endpoint: str):
        self.endpoint = endpoint

    def new_call(self, call: CallZammad):
        call_from, _, direction = self.parse_call(call)
        payload = {
            "event": "newCall",
            "from": call_from,
            "to": "",
            "user[]": call.agent_name,
            "direction": direction,
            "call_id": call.callid,
            "callid": call.callid,
        }

        return self.make_request(payload)

    def hangup(self, call: CallZammad):
        call_from, call_to, direction = self.parse_call(call)
        payload = {
            "event": "hangup",
            "from": call_from,
            "to": call_to,
            "direction": direction,
            "call_id": call.callid,
            "callid": call.callid,
            "cause": call.cause,
            "answeringNumber": call.agent
        }

        return self.make_request(payload)

    def answer(self, call: CallZammad):
        call_from, call_to, direction = self.parse_call(call)
        payload = {
            "event": "answer",
            "from": call_from,
            "to": call_to,
            "direction": direction,
            "call_id": call.callid,
            "callid": call.callid,
            "answeringNumber": call.agent
        }
        if call.direction == 'Inbound':
            payload['user[]'] = call.agent_name

        return self.make_request(payload)

    def make_request(self, payload: dict):
        resp = requests.post(self.endpoint, json=payload)
        try:
            if resp.status_code == 200:
                logging.info("Event sent to Zammad")
            else:
                logging.error("Error sending Event to Zammad: " + resp.text)
        except requests.RequestException as err:
            logging.error("Error sending Event to Zammad:" + str(err))

    def parse_call(self, call: CallZammad) -> [str, str, str]:
        if call.direction == "Inbound":
            call_from = call.number
            direction = "in"
            try:
                call_to = call.agent
            except:
                call_to = ""
                call.agent = ""
        else:
            call_from = call.agent
            call_to = call.number
            direction = "out"

        if not call.agent_name:
            call.agent_name = ''

        return [call_from, call_to, direction]
