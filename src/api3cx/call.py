class Call3CX:
    def __init__(self, d: dict):
        self.Caller = d['Caller']
        self.Callee = d['Callee']
        self.Id = d['Id']
        self.Status = d['Status']  # Possible values: "Talking", "Transferring", "Routing"
