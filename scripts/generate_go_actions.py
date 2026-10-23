#!/usr/bin/env python3
"""Generate standalone Go MCP action metadata from the Python API clients."""

from __future__ import annotations

import argparse
import ast
import json
import re
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
SERVICES = ("radarr", "sonarr", "bazarr")
EXCLUDED = {
    "request",
    "get_api",
    "post_login",
    "get_login",
    "get_logout",
    "get_feed_v3_calendar_radarrics",
    "get_feed_v3_calendar_sonarrics",
    "get_ping",
    "get_content_path",
    "get_",
    "get_path",
}
COMPOSITES = {"lookup_movie", "add_movie", "lookup_series", "add_series"}
MUTATION_VERBS = {"POST", "PUT", "PATCH", "DELETE"}
DESTRUCTIVE_TOKENS = {
    "delete",
    "remove",
    "purge",
    "drop",
    "restart",
    "shutdown",
    "restore",
}


def _name(node: ast.AST) -> str | None:
    return node.id if isinstance(node, ast.Name) else None


def _literal(node: ast.AST) -> Any:
    try:
        return ast.literal_eval(node)
    except (ValueError, TypeError):
        return None


def _path_template(node: ast.AST) -> tuple[str, list[str]]:
    if isinstance(node, ast.JoinedStr):
        path = ""
        names: list[str] = []
        for value in node.values:
            if isinstance(value, ast.Constant) and isinstance(value.value, str):
                path += value.value
            elif isinstance(value, ast.FormattedValue) and isinstance(
                value.value, ast.Name
            ):
                name = value.value.id
                path += "{" + name + "}"
                names.append(name)
            else:
                raise ValueError("unsupported dynamic endpoint expression")
        return path, names
    value = _literal(node)
    if isinstance(value, str):
        return value, re.findall(r"\{([A-Za-z_][A-Za-z0-9_]*)\}", value)
    raise ValueError("endpoint must be a literal or simple f-string")


def _dictionary_mapping(node: ast.AST, locals_: dict[str, ast.AST]) -> dict[str, str]:
    if isinstance(node, ast.Constant) and node.value is None:
        return {}
    if isinstance(node, ast.Name):
        node = locals_.get(node.id, node)
    if not isinstance(node, ast.Dict):
        raise ValueError(
            "request params/body must be a literal dictionary or local dictionary"
        )
    mapped: dict[str, str] = {}
    for key, value in zip(node.keys, node.values, strict=True):
        key_value = _literal(key) if key is not None else None
        value_name = _name(value)
        if not isinstance(key_value, str) or value_name is None:
            raise ValueError(
                "query/body mapping must map string keys to method parameters"
            )
        mapped[value_name] = key_value
    return mapped


def _defaults(function: ast.FunctionDef) -> dict[str, Any]:
    names = [arg.arg for arg in function.args.args if arg.arg != "self"]
    defaults: dict[str, Any] = {}
    if function.args.defaults:
        for name, value in zip(
            names[-len(function.args.defaults) :], function.args.defaults, strict=True
        ):
            defaults[name] = _literal(value)
    return defaults


def _required(function: ast.FunctionDef) -> list[str]:
    names = [arg.arg for arg in function.args.args if arg.arg != "self"]
    optional = len(function.args.defaults)
    required = names[: len(names) - optional] if optional else names
    return required


def _action(service: str, function: ast.FunctionDef) -> dict[str, Any]:
    locals_: dict[str, ast.AST] = {}
    requests: list[ast.Call] = []
    for node in ast.walk(function):
        if isinstance(node, (ast.Assign, ast.AnnAssign)):
            value = node.value
            targets = node.targets if isinstance(node, ast.Assign) else [node.target]
            for target in targets:
                if isinstance(target, ast.Name):
                    locals_[target.id] = value
        if (
            isinstance(node, ast.Call)
            and isinstance(node.func, ast.Attribute)
            and node.func.attr == "request"
        ):
            requests.append(node)
    if len(requests) != 1:
        if function.name == "search_series_subtitles" and service == "bazarr":
            return {
                "name": function.name,
                "method": "POST",
                "path": "/episodes/search",
                "path_params": [],
                "query_params": {},
                "body_params": {"episode_id": "episodeId"},
                "defaults": _defaults(function),
                "required_params": _required(function),
                "destructive": True,
                "variant": {
                    "parameter": "episode_id",
                    "truthy": True,
                    "ignore_params": ["series_id"],
                    "otherwise": {
                        "path": "/series/search",
                        "body_params": {"series_id": "seriesId"},
                    },
                },
            }
        raise ValueError(f"{function.name}: expected exactly one direct request() call")
    call = requests[0]
    method_node = call.args[0] if call.args else None
    path_node = call.args[1] if len(call.args) > 1 else None
    method = _literal(method_node) if method_node is not None else None
    if not isinstance(method, str):
        raise ValueError(f"{function.name}: HTTP method must be a string literal")
    path, path_params = _path_template(path_node)
    if service in {"radarr", "sonarr"}:
        if path.startswith("/api/v3/"):
            path = path[len("/api/v3") :]
        elif path.startswith("/api/"):
            path = path[len("/api") :]
        else:
            raise ValueError(f"{function.name}: endpoint is outside /api/v3 domain")
    else:
        if not path.startswith("/api/"):
            raise ValueError(f"{function.name}: endpoint is outside Bazarr /api domain")
        path = path[len("/api") :]

    keywords = {keyword.arg: keyword.value for keyword in call.keywords if keyword.arg}
    try:
        query_params = (
            _dictionary_mapping(keywords["params"], locals_)
            if "params" in keywords
            else {}
        )
    except ValueError as exc:
        raise ValueError(f"{function.name}: {exc}") from exc
    body_name = _name(keywords.get("data", ast.Constant(None)))
    arguments = {arg.arg for arg in function.args.args if arg.arg != "self"}
    body_direct = (
        body_name if body_name in arguments and body_name not in locals_ else ""
    )
    try:
        body_params = (
            _dictionary_mapping(keywords["data"], locals_)
            if "data" in keywords and not body_direct
            else {}
        )
    except ValueError as exc:
        raise ValueError(f"{function.name}: {exc}") from exc
    if "data" in keywords and not isinstance(
        keywords["data"], (ast.Name, ast.Dict, ast.Constant)
    ):
        raise ValueError(f"{function.name}: unsupported body expression")
    # Methods that pass data=data expose the caller's whole JSON object as body.
    defaults = _defaults(function)
    allowed = arguments
    used = set(path_params) | set(query_params) | set(body_params)
    if body_direct:
        used.add(body_direct)
    unknown = used - allowed
    if unknown:
        raise ValueError(
            f"{function.name}: request mapping refers to non-argument(s) {sorted(unknown)}"
        )
    params = {
        "name": function.name,
        "method": method,
        "path": path,
        "path_params": path_params,
        "query_params": query_params,
        "body_params": body_params,
        "body_direct": body_direct,
        "defaults": defaults,
        "required_params": _required(function),
        "destructive": method in MUTATION_VERBS
        or bool(DESTRUCTIVE_TOKENS & set(function.name.split("_")))
        or function.name.startswith("put_config_"),
    }
    return params


def generate(service: str) -> list[dict[str, Any]]:
    source = ROOT / "arr_mcp" / "api" / f"api_client_{service}.py"
    tree = ast.parse(source.read_text(encoding="utf-8"), filename=str(source))
    client = next(
        node
        for node in tree.body
        if isinstance(node, ast.ClassDef) and node.name == "Api"
    )
    actions: list[dict[str, Any]] = []
    for function in client.body:
        if not isinstance(function, ast.FunctionDef) or function.name.startswith("_"):
            continue
        if function.name in EXCLUDED:
            continue
        if function.name in COMPOSITES:
            actions.append({"name": function.name, "composite": True})
            continue
        actions.append(_action(service, function))
    names = [action["name"] for action in actions]
    if len(names) != len(set(names)):
        raise ValueError(f"{service}: duplicate public action name")
    return sorted(actions, key=lambda action: action["name"])


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--service", choices=SERVICES, action="append")
    parser.add_argument(
        "--check", action="store_true", help="fail if checked-in metadata differs"
    )
    args = parser.parse_args()
    for service in args.service or SERVICES:
        actions = generate(service)
        target = ROOT / "internal" / service / "actions.json"
        rendered = json.dumps(actions, indent=2, sort_keys=True) + "\n"
        if args.check:
            if not target.exists() or target.read_text(encoding="utf-8") != rendered:
                raise SystemExit(
                    f"{service}: action catalog is stale; rerun scripts/generate_go_actions.py"
                )
        else:
            target.write_text(rendered, encoding="utf-8")
        print(
            f"{service}: {'verified' if args.check else 'generated'} {len(actions)} actions"
        )


if __name__ == "__main__":
    main()
