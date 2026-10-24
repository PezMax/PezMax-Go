from .errors import AgentError

SEARCH_TOOL = {
    "type": "function",
    "function": {
        "name": "search_resources",
        "description": "搜索 PezMax 已上架的真实资料、电子书和学习书签。每次使用一个简短关键词；只检索目录信息，不读取文件正文。",
        "parameters": {
            "type": "object",
            "properties": {
                "kinds": {"type": "array", "items": {"type": "string", "enum": ["file", "ebook", "bookmark"]}},
                "keyword": {"type": "string", "description": "简短关键词，如线性代数；不要整句搜索"},
                "subject": {"type": "string", "description": "学科全称，如高等数学"},
                "school": {"type": "string", "description": "学校全称，仅对资料有效"},
                "yearFrom": {"type": "integer", "description": "起始年份，包含此年，仅对资料有效"},
                "yearTo": {"type": "integer", "description": "结束年份，包含此年，仅对资料有效"},
                "fileType": {"type": "integer", "enum": [1, 2, 3, 4, 5, 6],
                             "description": "仅资料：1期末、2期中、3补考、4资料、5其他学校、6神秘文件"},
            },
            "required": ["kinds", "keyword"],
            "additionalProperties": False,
        },
    },
}


def search_arguments(value) -> dict:
    allowed = {"kinds", "keyword", "subject", "school", "yearFrom", "yearTo", "fileType"}
    if not isinstance(value, dict) or set(value) - allowed:
        raise AgentError("搜索包含不支持的筛选条件")
    kinds = value.get("kinds", ["file", "ebook", "bookmark"])
    if (not isinstance(kinds, list) or not 1 <= len(kinds) <= 3
            or any(kind not in ("file", "ebook", "bookmark") for kind in kinds)):
        raise AgentError("资源类型无效")
    args = {"kinds": list(dict.fromkeys(kinds))}
    for key in ("keyword", "subject", "school"):
        text = value.get(key, "")
        if not isinstance(text, str) or len(text.strip()) > 100:
            raise AgentError("搜索关键词格式不正确或过长")
        args[key] = text.strip()
    for key in ("yearFrom", "yearTo", "fileType"):
        number = value.get(key)
        if number is None:
            continue
        if type(number) is not int:
            raise AgentError("年份和类型必须为整数")
        low, high = (1, 6) if key == "fileType" else (1900, 2100)
        if not low <= number <= high:
            raise AgentError("资料类型无效" if key == "fileType" else "年份需在1900到2100之间")
        args[key] = number
    if args.get("yearFrom", 1900) > args.get("yearTo", 2100):
        raise AgentError("起始年份不能晚于结束年份")
    if not any(args[key] for key in ("keyword", "subject", "school")):
        raise AgentError("请至少提供关键词、学科或学校之一")
    return args
