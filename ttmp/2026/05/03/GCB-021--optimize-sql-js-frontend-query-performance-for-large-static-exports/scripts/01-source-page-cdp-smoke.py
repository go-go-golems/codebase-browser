#!/usr/bin/env python3
"""Smoke-test a static codebase-browser source page through Chromium CDP.

This is intentionally dependency-light: it uses the system Chromium plus the
`requests` and `websocket-client` Python packages already available on the
machine. It captures console messages so `?debugSql` output can be reviewed
without opening DevTools manually.
"""
from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import tempfile
import time
from typing import Any

import requests
import websocket


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("url", help="URL to open, usually with ?debugSql and a #/source/... route")
    parser.add_argument("--timeout", type=float, default=30.0)
    parser.add_argument("--ready-text", action="append", default=["Used by (", "Uses ("], help="Body text that must appear before the page is considered ready. Can be repeated.")
    parser.add_argument("--chromium", default=shutil.which("chromium") or shutil.which("chromium-browser") or shutil.which("google-chrome"))
    return parser.parse_args()


class CDP:
    def __init__(self, ws_url: str):
        self.ws = websocket.create_connection(ws_url, timeout=1)
        self.next_id = 1

    def call(self, method: str, params: dict[str, Any] | None = None) -> dict[str, Any]:
        msg_id = self.next_id
        self.next_id += 1
        self.ws.send(json.dumps({"id": msg_id, "method": method, "params": params or {}}))
        while True:
            raw = self.ws.recv()
            msg = json.loads(raw)
            if msg.get("id") == msg_id:
                if "error" in msg:
                    raise RuntimeError(f"CDP {method} failed: {msg['error']}")
                return msg.get("result", {})

    def recv_event(self, timeout: float = 0.2) -> dict[str, Any] | None:
        old_timeout = self.ws.gettimeout()
        self.ws.settimeout(timeout)
        try:
            raw = self.ws.recv()
            return json.loads(raw)
        except Exception:
            return None
        finally:
            self.ws.settimeout(old_timeout)

    def close(self) -> None:
        self.ws.close()


def wait_for_debugger(port: int, timeout: float) -> str:
    deadline = time.time() + timeout
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            tabs = requests.get(f"http://127.0.0.1:{port}/json", timeout=0.5).json()
            for tab in tabs:
                if tab.get("type") == "page":
                    return tab["webSocketDebuggerUrl"]
        except Exception as exc:  # pragma: no cover - diagnostic only
            last_error = exc
        time.sleep(0.1)
    raise RuntimeError(f"Chromium did not expose CDP in time: {last_error}")


def main() -> int:
    args = parse_args()
    if not args.chromium:
        raise SystemExit("No chromium executable found")

    profile = tempfile.mkdtemp(prefix="gcb-cdp-profile-")
    port = 9229
    proc = subprocess.Popen([
        args.chromium,
        "--headless=new",
        "--no-sandbox",
        "--disable-gpu",
        f"--user-data-dir={profile}",
        f"--remote-debugging-port={port}",
        "--remote-allow-origins=*",
        "about:blank",
    ], stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, text=True)

    logs: list[str] = []
    started = time.time()
    try:
        ws_url = wait_for_debugger(port, args.timeout)
        cdp = CDP(ws_url)
        cdp.call("Runtime.enable")
        cdp.call("Page.enable")
        cdp.call("Page.navigate", {"url": args.url})

        found_ready = False
        deadline = time.time() + args.timeout
        while time.time() < deadline:
            event = cdp.recv_event(0.2)
            if event and event.get("method") == "Runtime.consoleAPICalled":
                params = event.get("params", {})
                args_ = params.get("args", [])
                rendered = []
                for arg in args_:
                    if "value" in arg:
                        rendered.append(str(arg["value"]))
                    elif "preview" in arg:
                        props = arg["preview"].get("properties", [])
                        rendered.append("{" + ", ".join(f"{p.get('name')}={p.get('value')}" for p in props) + "}")
                    else:
                        rendered.append(str(arg.get("description", "")))
                logs.append(f"[{params.get('type')}] " + " ".join(rendered))

            ready_text_json = json.dumps(args.ready_text)
            result = cdp.call("Runtime.evaluate", {
                "expression": f"(Boolean(document.querySelector('[data-part=source-view]')) && {ready_text_json}.every(t => document.body.innerText.includes(t))) || document.body.innerText.includes('Failed')",
                "returnByValue": True,
            })
            found_ready = bool(result.get("result", {}).get("value"))
            if found_ready:
                break

        text = cdp.call("Runtime.evaluate", {
            "expression": "document.body.innerText.slice(0, 2000)",
            "returnByValue": True,
        }).get("result", {}).get("value", "")
        cdp.close()

        elapsed = time.time() - started
        print(json.dumps({
            "url": args.url,
            "elapsedSeconds": round(elapsed, 3),
            "ready": found_ready,
            "bodyPreview": text,
            "console": logs,
        }, indent=2))
        return 0 if found_ready and "Failed" not in text else 2
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
        shutil.rmtree(profile, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
