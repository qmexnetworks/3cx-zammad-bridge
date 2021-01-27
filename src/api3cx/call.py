class Call3CX:
    Caller: str
    Callee: str
    Id: int
    Status: str  # Possible values: "Talking", "Transferring", "Routing"

    def __init__(self, d: dict):
        self.Caller = d['Caller']
        self.Callee = d['Callee']
        self.Id = d['Id']
        self.Status = d['Status']
