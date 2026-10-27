# Reading contract evaluation

All fixtures are explicitly synthetic math/software text. No user documents,
credentials, network, hosted traces, or real model calls are used.

From `PezMax-Agent`:

```powershell
.venv\Scripts\python -m pezmax_agent.evaluation --output evals/results/local.json
```

Ten scenarios run across PDF/EPUB and QA/summary/outline (60 cases). The real
LangGraph executes against scripted model responses. The independent grader
checks released field sets, integer locations and exact normalized source
quotes, plus the expected answer count and call budget. An error exits nonzero.

The report compares raw first output, validation alone, and validation with one
bounded corrective attempt. Recovery is a scripted orchestration metric, not
real model accuracy. No grader checks semantic entailment. Measured durations
reflect local execution with a fake model, not production latency or cost.

Generated reports under `results/` are local artifacts. Published results must
retain the dataset version and these scope limitations. Runtime diagnostics
contain fixed node names, flags and numbers; reports add synthetic case IDs.
