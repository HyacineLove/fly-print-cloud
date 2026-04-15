import json
import sys
import time
from typing import Any, Dict, List

import requests

from cloud_test_utils import (
    auth_headers,
    ensure_results_dir,
    get_admin_token,
    get_base_url,
    get_edge_token,
    timestamp_slug,
)


def run_case(
    session: requests.Session,
    name: str,
    method: str,
    url: str,
    *,
    expected_status: int = 200,
    headers: Dict[str, str] | None = None,
    data: Dict[str, Any] | None = None,
    json_body: Dict[str, Any] | None = None,
) -> Dict[str, Any]:
    response = session.request(
        method=method,
        url=url,
        headers=headers,
        data=data,
        json=json_body,
        timeout=15,
    )
    record: Dict[str, Any] = {
        "name": name,
        "method": method,
        "url": url,
        "status_code": response.status_code,
        "expected_status": expected_status,
        "ok": response.status_code == expected_status,
    }
    try:
        record["body"] = response.json()
    except Exception:
        record["body"] = response.text[:500]
    time.sleep(0.25)
    return record


def main() -> int:
    base_url = get_base_url()
    results: List[Dict[str, Any]] = []
    warnings: List[str] = []

    with requests.Session() as session:
        admin_token = get_admin_token(session)
        edge_token = None
        try:
            edge_token = get_edge_token(session)
        except Exception as exc:
            warnings.append(f"Edge client token skipped: {exc}")

        results.append(run_case(session, "auth_mode", "GET", f"{base_url}/auth/mode"))
        results.append(
            run_case(
                session,
                "auth_userinfo",
                "GET",
                f"{base_url}/auth/userinfo",
                headers=auth_headers(admin_token),
            )
        )
        results.append(
            run_case(
                session,
                "api_health",
                "GET",
                f"{base_url}/api/v1/health",
            )
        )
        results.append(
            run_case(
                session,
                "dashboard_trends",
                "GET",
                f"{base_url}/api/v1/admin/dashboard/trends",
                headers=auth_headers(admin_token),
            )
        )
        results.append(
            run_case(
                session,
                "edge_nodes",
                "GET",
                f"{base_url}/api/v1/admin/edge-nodes",
                headers=auth_headers(admin_token),
            )
        )
        results.append(
            run_case(
                session,
                "printers",
                "GET",
                f"{base_url}/api/v1/admin/printers",
                headers=auth_headers(admin_token),
            )
        )
        results.append(
            run_case(
                session,
                "print_jobs",
                "GET",
                f"{base_url}/api/v1/admin/print-jobs",
                headers=auth_headers(admin_token),
            )
        )
        if edge_token:
            edge_case = run_case(
                session,
                "edge_token_userinfo",
                "GET",
                f"{base_url}/auth/userinfo",
                headers=auth_headers(edge_token),
            )
            if edge_case["ok"]:
                results.append(edge_case)
            else:
                warnings.append(
                    f"Edge token diagnostic returned {edge_case['status_code']}: {edge_case['body']}"
                )

    passed = sum(1 for item in results if item["ok"])
    failed = len(results) - passed
    summary = {
        "base_url": base_url,
        "passed": passed,
        "failed": failed,
        "warnings": warnings,
        "results": results,
    }

    output_path = ensure_results_dir() / f"cloud_smoke_{timestamp_slug()}.json"
    output_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")

    print(f"Cloud smoke test finished: {passed} passed, {failed} failed")
    print(f"Result file: {output_path}")
    for warning in warnings:
        print(f"[WARN] {warning}")
    for item in results:
        marker = "PASS" if item["ok"] else "FAIL"
        print(f"[{marker}] {item['name']} -> {item['status_code']}")

    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
