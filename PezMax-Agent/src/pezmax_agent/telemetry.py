"""Ephemeral, allowlisted execution measurements. Never records graph state."""
import inspect
from functools import wraps
from time import perf_counter

NODES = {
    "search": {"model", "search", "finalize"},
    "upload": {"suggest", "validate"},
    "reading": {"retrieve", "answer", "validate", "repair"},
}
COUNTERS = {"modelCalls", "searches", "selectedChunks", "verifiedAnswers", "verifiedCitations", "rejectedAnswers", "validationFailures", "repairAttempts"}


class RunMetrics:
    def __init__(self, task, clock=perf_counter):
        if task not in NODES:
            raise ValueError("Unknown task")
        self.task, self.clock = task, clock
        self.started, self.steps = clock(), []

    def wrap(self, name, function):
        if name not in NODES[self.task]:
            raise ValueError("Unknown node")

        @wraps(function)
        async def measured(state):
            start, ok = self.clock(), False
            try:
                value = function(state)
                result = await value if inspect.isawaitable(value) else value
                ok = True
                return result
            finally:
                self.steps.append({"node": name, "durationMs": max(0, round((self.clock() - start) * 1000)), "ok": ok})
        return measured

    def snapshot(self, **counters):
        if any(key not in COUNTERS or type(value) is not int or value < 0 for key, value in counters.items()):
            raise ValueError("Only allowlisted numeric measurements may be exported")
        return {"version": 1, "task": self.task,
                "durationMs": max(0, round((self.clock() - self.started) * 1000)),
                "steps": [dict(step) for step in self.steps], **counters}
