from uuid import UUID


class CallZammad:
    callid: UUID
    direction: str  # Either "Inbound" or "Outbound"
    number: str
    status: str

    cause: str or None = None
    agent: str or None = None
    agent_name: str or None = None
