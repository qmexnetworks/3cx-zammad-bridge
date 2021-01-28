from uuid import UUID


class CallZammad:
    def __init__(self):
        self.callid = None
        self.direction = None# Either "Inbound" or "Outbound"
        self.number = None
        self.status = None
        self.cause = None
        self.agent = None
        self.agent_name = None
