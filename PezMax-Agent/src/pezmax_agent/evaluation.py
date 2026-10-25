"""Offline, deterministic contract evaluation, including a repair ablation.

No keys, network, user documents or hosted tracing. This measures orchestration
against scripted model outputs; it is deliberately not an LLM quality benchmark.
"""
import argparse
import asyncio
import json
import platform
from hashlib import sha256
from importlib.metadata import version
from pathlib import Path

from .errors import AgentError
from .reading import parse_reading, run_reading_assistant, select_context, validate_answer


class ScriptedModel:
    def __init__(self, responses):
        self.responses, self.calls = responses, 0

    async def complete(self, messages, **options):
        if self.calls >= 2 or options.get("tools") != [] or options.get("allow_tools") is not False:
            raise AssertionError("Reading exceeded its model/tool budget")
        message = self.responses[min(self.calls, len(self.responses) - 1)]
        self.calls += 1
        return message


def substitute(value, key):
    if isinstance(value, dict):
        return {key if name == "location" else name: substitute(item, key) for name, item in value.items()}
    if isinstance(value, list):
        return [substitute(item, key) for item in value]
    return value


def unsafe_count(answers, chunks, key):
    """Independent grader: exact types, output keys and source substring checks."""
    invalid = 0
    for answer in answers:
        safe = isinstance(answer, dict) and set(answer) == {"text", "citations"}
        if safe:
            safe = isinstance(answer["text"], str) and 1 <= len(answer["text"].strip()) <= 1600
            citations = answer["citations"]
            safe = safe and isinstance(citations, list) and 1 <= len(citations) <= 3
        if safe:
            for cite in citations:
                if not isinstance(cite, dict) or set(cite) != {key, "quote"} or type(cite[key]) is not int or not isinstance(cite["quote"], str):
                    safe = False
                    break
                quote = " ".join(cite["quote"].split())
                if not 4 <= len(quote) <= 600 or not any(chunk[key] == cite[key] and quote in chunk["text"] for chunk in chunks):
                    safe = False
                    break
        invalid += int(not safe)
    return invalid


async def evaluate(dataset):
    rows = []
    for file_format in ("pdf", "epub"):
        key = "chapter" if file_format == "epub" else "page"
        for mode in ("qa", "summary", "outline"):
            for case in dataset["cases"]:
                reading = parse_reading({"format": file_format, "mode": mode, "question": "What does the derivative measure?",
                    "chapters" if file_format == "epub" else "pages": [{key: 2, "text": case.get("text", dataset["text"])}]})
                responses = [{"content": item if isinstance(item, str) else json.dumps(substitute(item, key))} for item in case["responses"]]
                model = ScriptedModel(responses)
                result = await run_reading_assistant(model, reading)
                chunks, _ = select_context(reading)
                try:
                    baseline = json.loads(responses[0]["content"])["answers"]
                except (ValueError, TypeError, KeyError):
                    baseline = []
                try:
                    validated_only = validate_answer(responses[0], chunks, key)
                except AgentError:
                    validated_only = []
                released_unsafe = unsafe_count(result["answers"], chunks, key)
                passed = (len(result["answers"]) == case["expectedAnswers"] and model.calls == case["expectedCalls"] and released_unsafe == 0)
                rows.append({"id": f"{file_format}/{mode}/{case['id']}", "passed": passed,
                    "rawUnsafeAnswers": unsafe_count(baseline, chunks, key),
                    "validateOnlyAnswers": len(validated_only), "validateOnlyUnsafeAnswers": unsafe_count(validated_only, chunks, key),
                    "releasedAnswers": len(result["answers"]), "releasedUnsafeAnswers": released_unsafe,
                    "recoveryEligible": not validated_only and case["expectedAnswers"] > 0,
                    "recovered": not validated_only and len(result["answers"]) == case["expectedAnswers"] and case["expectedAnswers"] > 0,
                    "diagnostics": result["diagnostics"]})
    return {"suite": "reading-contracts-v1", "scope": "offline scripted orchestration; not real-model accuracy or latency",
            "environment": {"python": platform.python_version(), "langgraph": version("langgraph"), "pydantic": version("pydantic")},
            "summary": {"cases": len(rows), "passed": sum(row["passed"] for row in rows),
                "rawUnsafeAnswers": sum(row["rawUnsafeAnswers"] for row in rows),
                "validateOnlyUnsafeAnswers": sum(row["validateOnlyUnsafeAnswers"] for row in rows),
                "releasedUnsafeAnswers": sum(row["releasedUnsafeAnswers"] for row in rows),
                "recoveryEligible": sum(row["recoveryEligible"] for row in rows), "recovered": sum(row["recovered"] for row in rows),
                "modelCalls": sum(row["diagnostics"]["modelCalls"] for row in rows)}, "cases": rows}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cases", type=Path, default=Path(__file__).resolve().parents[2] / "evals" / "reading-cases.json")
    parser.add_argument("--output", type=Path, help="Optional explicit destination; contains synthetic case IDs and numeric diagnostics only")
    args = parser.parse_args()
    source = args.cases.read_text(encoding="utf-8")
    dataset = json.loads(source)
    report = asyncio.run(evaluate(dataset))
    # Universal newline decoding makes the dataset ID stable on Windows/Unix.
    report["dataset"] = {"version": dataset["version"], "sha256": sha256(source.encode("utf-8")).hexdigest()}
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report["summary"], ensure_ascii=False))
    return 0 if report["summary"]["cases"] and report["summary"]["passed"] == report["summary"]["cases"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
