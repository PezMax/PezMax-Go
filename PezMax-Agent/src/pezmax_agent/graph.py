import json
from typing import TypedDict

from langgraph.graph import END, START, StateGraph
from langsmith import tracing_context

from .contracts import bounded_history, json_text
from .errors import AgentError

MAX_MODEL_ROUNDS = 5
MAX_SEARCHES = 8


class AssistantState(TypedDict):
    messages: list[dict]
    resources: dict[str, dict]
    reports: list[dict]
    rounds: int
    searches: int
    content: str


def build_graph(model, catalog, on_progress=lambda event: None):
    """Dependencies live outside graph state: credentials cannot enter history
    or LangGraph state streams. No checkpoint or tracing service is configured.
    """
    async def call_model(state: AssistantState):
        on_progress({"status": "正在整理检索结果…" if state["rounds"] else "正在理解你的需求…"})
        message = await model.complete(state["messages"], allow_tools=(
            state["rounds"] < MAX_MODEL_ROUNDS - 1 and state["searches"] < MAX_SEARCHES))
        return {"messages": [*state["messages"], message], "rounds": state["rounds"] + 1}

    def after_model(state: AssistantState):
        calls = state["messages"][-1].get("tool_calls")
        return "search" if calls and state["rounds"] < MAX_MODEL_ROUNDS else "finalize"

    async def search(state: AssistantState):
        messages, resources = list(state["messages"]), dict(state["resources"])
        reports, count = list(state["reports"]), state["searches"]
        for call in messages[-1]["tool_calls"]:
            function = call["function"]
            if function["name"] != "search_resources":
                result = {"error": "不支持此工具；只能使用 search_resources 检索目录"}
            elif count >= MAX_SEARCHES:
                result = {"error": "达到本轮8次查询上限，请向用户总结已有结果"}
            else:
                count += 1
                on_progress({"status": f"正在检索目录 · 第 {count} 次查询…"})
                try:
                    arguments = json.loads(function["arguments"])
                    found = await catalog.search(arguments)
                    for item in found["resources"]:
                        resources[item["id"]] = item
                    result = found["modelResult"]
                    reports.extend(result["reports"])
                    on_progress({"resources": list(resources.values()), "report": result["reports"]})
                except (ValueError, TypeError):
                    result = {"error": "搜索参数不是有效JSON，请修正后重试"}
                except AgentError as error:
                    if error.code == "catalog_auth_expired":
                        raise
                    result = {"error": str(error)}
            messages.append({"role": "tool", "tool_call_id": call["id"], "content": json_text(result)})
        return {"messages": messages, "resources": resources, "reports": reports, "searches": count}

    def finalize(state: AssistantState):
        messages = list(state["messages"])
        last = messages[-1]
        if last.get("tool_calls"):
            # A model ignoring tool_choice=none must not leave dangling calls
            # in follow-up history or trigger another graph iteration.
            for call in last["tool_calls"]:
                messages.append({"role": "tool", "tool_call_id": call["id"],
                                 "content": json_text({"error": "已达到本轮查询上限"})})
            content = "已达到本轮查询上限。你可以先查看已找到的资源，或缩小范围后继续提问。"
            messages.append({"role": "assistant", "content": content})
        else:
            content = last["content"] or "未收到文字回复，可调整条件重试。"
            messages[-1] = {**last, "content": content}
        return {"messages": messages, "content": content}

    builder = StateGraph(AssistantState)
    builder.add_node("model", call_model)
    builder.add_node("search", search)
    builder.add_node("finalize", finalize)
    builder.add_edge(START, "model")
    builder.add_conditional_edges("model", after_model, {"search": "search", "finalize": "finalize"})
    builder.add_edge("search", "model")
    builder.add_edge("finalize", END)
    return builder.compile()


async def run_assistant(model, catalog, *, history, question, on_progress=lambda event: None):
    if not isinstance(question, str) or not 1 <= len(question.strip()) <= 2000:
        raise AgentError("请输入1到2000字的需求")
    graph = build_graph(model, catalog, on_progress)
    # Explicit context also covers library callers, not just the stdio entry.
    # An inherited LangSmith setting must not export graph state or callbacks.
    with tracing_context(enabled=False):
        state = await graph.ainvoke({
            "messages": [*bounded_history(history), {"role": "user", "content": question.strip()}],
            "resources": {}, "reports": [], "rounds": 0, "searches": 0, "content": "",
        }, config={"recursion_limit": 16})
    return {"messages": bounded_history(state["messages"]), "content": state["content"],
            "resources": list(state["resources"].values()), "reports": state["reports"]}
