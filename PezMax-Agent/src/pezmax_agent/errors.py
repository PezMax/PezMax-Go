class AgentError(Exception):
    """Only application-authored messages may cross the process boundary."""

    def __init__(self, message: str, code: str = "agent_error"):
        super().__init__(message)
        self.code = code
