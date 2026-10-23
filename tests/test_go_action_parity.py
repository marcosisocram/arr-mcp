import json
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def test_go_action_catalog_matches_python_api_clients():
    result = subprocess.run(
        [sys.executable, "scripts/generate_go_actions.py", "--check"],
        cwd=ROOT,
        check=False,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stdout + result.stderr


def test_catalog_counts_and_ui_exclusions_are_explicit():
    expected = {"radarr": 230, "sonarr": 227, "bazarr": 25}
    for service, count in expected.items():
        catalog_path = ROOT / "internal" / service / "actions.json"
        actions = json.loads(catalog_path.read_text(encoding="utf-8"))
        assert len(actions) == count
        names = {action["name"] for action in actions}
        assert len(names) == count
        assert "request" not in names
        assert "get_api" not in names
        assert "get_ping" not in names
        assert "get_content_path" not in names
        assert "get_" not in names
        assert "get_path" not in names
        if service in {"radarr", "sonarr"}:
            assert "get_feed_v3_calendar_" + service + "ics" not in names
            assert "get_login" not in names
            assert "get_logout" not in names
            assert "post_login" not in names


def test_every_generated_route_has_supported_method_and_matching_path_params():
    for service in ("radarr", "sonarr", "bazarr"):
        actions = json.loads(
            (ROOT / "internal" / service / "actions.json").read_text(encoding="utf-8")
        )
        for action in actions:
            if action.get("composite"):
                continue
            assert action["method"] in {"GET", "POST", "PUT", "PATCH", "DELETE"}
            assert set(
                re.findall(r"\{([A-Za-z_][A-Za-z0-9_]*)\}", action["path"])
            ) == set(action["path_params"])
            assert isinstance(action["query_params"], dict)
            assert isinstance(action["body_params"], dict)
