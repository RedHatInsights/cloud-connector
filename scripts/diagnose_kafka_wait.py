#!/usr/bin/env python3
"""Diagnose Cloud Connector Kafka wait-time SLO burns from Kibana/ES dumps.

Takes one or more Elasticsearch _search JSON responses (Dev Tools / Inspect)
and prints the same conclusion we reach by hand:

  1. Is pickup wait (now - date_received) actually over the 1s SLO?
  2. Which client_id / partition / pod concentrated the slow messages?
  3. What were those messages (connection-status online vs other)?
  4. Were they dropped as duplicate/old, and is payload `sent` stale?

Usage:
  python3 scripts/diagnose_kafka_wait.py dump.json [dump2.json ...]
  python3 scripts/diagnose_kafka_wait.py ~/Downloads/Harper2/*.txt

The dump must be the raw ES response (has hits.hits). Multiple files are
merged and de-duplicated by _id so you can combine a volume dump with a
client-id follow-up dump.
"""

from __future__ import annotations

import argparse
import json
import math
import re
import statistics
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

SLO_SECONDS = 1.0
PICKUP_MSG = "Read message off of kafka topic"
PAYLOAD_PREFIX = "Received control message on topic:"
DUP_MSG = "ignoring message - duplicate or old message"
ONLINE_MSG = "handling online connection-status message"
OFFLINE_MSG = "handling offline connection-status message"
CONTROL_GOT_PREFIX = "Got a control message:"

TZ_TAIL = re.compile(r"([+-]\d{2}:\d{2})$")
PAYLOAD_JSON = re.compile(r"Message:\s*(\{.*\})\s*$", re.DOTALL)


def parse_iso(value: Any) -> datetime | None:
    if not value:
        return None
    text = str(value).strip()
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    if "." in text:
        head, tail = text.split(".", 1)
        match = TZ_TAIL.search(tail)
        if match:
            frac = "".join(c for c in tail[: match.start()] if c.isdigit())[:6].ljust(6, "0")
            text = f"{head}.{frac}{match.group(1)}"
        else:
            frac = "".join(c for c in tail if c.isdigit())[:6].ljust(6, "0")
            text = f"{head}.{frac}+00:00"
    try:
        parsed = datetime.fromisoformat(text)
    except ValueError:
        return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed


def percentile(sorted_vals: list[float], pct: float) -> float:
    if not sorted_vals:
        return float("nan")
    if len(sorted_vals) == 1:
        return sorted_vals[0]
    idx = pct / 100.0 * (len(sorted_vals) - 1)
    lo = math.floor(idx)
    hi = math.ceil(idx)
    if lo == hi:
        return sorted_vals[int(idx)]
    weight = idx - lo
    return sorted_vals[lo] * (1 - weight) + sorted_vals[hi] * weight


def load_search_response(path: Path) -> dict[str, Any]:
    with path.open() as handle:
        data = json.load(handle)
    if not isinstance(data, dict) or "hits" not in data:
        raise ValueError(f"{path} is not an ES _search response (missing hits)")
    return data


def hits_total(data: dict[str, Any]) -> int:
    total = data.get("hits", {}).get("total", 0)
    if isinstance(total, dict):
        return int(total.get("value") or 0)
    return int(total or 0)


def message_head(message: str) -> str:
    return (message or "").split("\n", 1)[0].strip()


class Dump:
    def __init__(self) -> None:
        self.file_totals: list[tuple[str, int, int]] = []
        self.hits: list[dict[str, Any]] = []

    def add_file(self, path: Path) -> None:
        data = load_search_response(path)
        returned = data.get("hits", {}).get("hits") or []
        self.file_totals.append((str(path), hits_total(data), len(returned)))
        seen = {h.get("_id") for h in self.hits if h.get("_id")}
        for hit in returned:
            hid = hit.get("_id")
            if hid and hid in seen:
                continue
            if hid:
                seen.add(hid)
            self.hits.append(hit)


def analyze(dump: Dump) -> dict[str, Any]:
    pickups: list[dict[str, Any]] = []
    payloads: list[dict[str, Any]] = []
    messages = Counter()
    dup_ids = Counter()
    online = offline = 0

    for hit in dump.hits:
        src = hit.get("_source") or {}
        msg = src.get("message") or ""
        head = message_head(msg)
        if head.startswith(CONTROL_GOT_PREFIX):
            messages[CONTROL_GOT_PREFIX] += 1
        elif head.startswith(PAYLOAD_PREFIX):
            messages[PAYLOAD_PREFIX] += 1
        else:
            messages[head[:160]] += 1

        if DUP_MSG in msg:
            dup_ids[src.get("message_id") or ""] += 1
        if ONLINE_MSG in msg:
            online += 1
        if OFFLINE_MSG in msg:
            offline += 1

        if head == PICKUP_MSG:
            ts = parse_iso(src.get("@timestamp"))
            received = parse_iso(src.get("date_received"))
            wait = None
            if ts and received:
                wait = (ts - received).total_seconds()
            pickups.append(
                {
                    "ts": ts,
                    "received": received,
                    "wait": wait,
                    "client_id": src.get("client_id"),
                    "partition": src.get("partition"),
                    "offset": src.get("offset"),
                    "pod": src.get("source_host"),
                    "mqtt_message_id": src.get("mqtt_message_id"),
                }
            )

        if head.startswith(PAYLOAD_PREFIX):
            match = PAYLOAD_JSON.search(msg)
            body = None
            if match:
                try:
                    body = json.loads(match.group(1))
                except json.JSONDecodeError:
                    body = None
            content = (body or {}).get("content") if isinstance(body, dict) else None
            facts = (content or {}).get("canonical_facts") if isinstance(content, dict) else None
            payloads.append(
                {
                    "ts": parse_iso(src.get("@timestamp")),
                    "client_id": src.get("client_id"),
                    "type": (body or {}).get("type") if isinstance(body, dict) else None,
                    "message_id": (body or {}).get("message_id") if isinstance(body, dict) else None,
                    "sent": parse_iso((body or {}).get("sent")) if isinstance(body, dict) else None,
                    "state": (content or {}).get("state") if isinstance(content, dict) else None,
                    "fqdn": (facts or {}).get("fqdn") if isinstance(facts, dict) else None,
                    "insights_id": (facts or {}).get("insights_id") if isinstance(facts, dict) else None,
                    "subscription_manager_id": (facts or {}).get("subscription_manager_id")
                    if isinstance(facts, dict)
                    else None,
                    "body": body,
                }
            )

    waits = sorted(p["wait"] for p in pickups if p["wait"] is not None)
    slow = [p for p in pickups if p["wait"] is not None and p["wait"] > SLO_SECONDS]

    by_client = Counter(p["client_id"] for p in pickups if p["client_id"])
    slow_by_client = Counter(p["client_id"] for p in slow if p["client_id"])
    by_partition = Counter(p["partition"] for p in pickups if p["partition"] is not None)
    slow_by_partition = Counter(p["partition"] for p in slow if p["partition"] is not None)
    by_pod = Counter(p["pod"] for p in pickups if p["pod"])

    client_waits: dict[str, list[float]] = defaultdict(list)
    for pickup in pickups:
        if pickup["client_id"] and pickup["wait"] is not None:
            client_waits[pickup["client_id"]].append(pickup["wait"])

    payload_types = Counter(p["type"] for p in payloads if p["type"])
    payload_states = Counter(p["state"] for p in payloads if p["state"])
    payload_fqdns = Counter(p["fqdn"] for p in payloads if p["fqdn"])
    payload_ids = [p["message_id"] for p in payloads if p["message_id"]]
    sent_times = [p["sent"] for p in payloads if p["sent"]]
    pickup_times = [p["ts"] for p in pickups if p["ts"]]
    received_times = [p["received"] for p in pickups if p["received"]]

    stale_sent = 0
    if sent_times and pickup_times:
        latest_pickup = max(pickup_times)
        stale_sent = sum(1 for sent in sent_times if (latest_pickup - sent).total_seconds() > 3600)

    hot_client = slow_by_client.most_common(1)[0][0] if slow_by_client else (
        by_client.most_common(1)[0][0] if by_client else None
    )
    hot_partition = slow_by_partition.most_common(1)[0][0] if slow_by_partition else None

    return {
        "file_totals": dump.file_totals,
        "returned_hits": len(dump.hits),
        "messages": messages,
        "pickups": pickups,
        "payloads": payloads,
        "waits": waits,
        "slow": slow,
        "by_client": by_client,
        "slow_by_client": slow_by_client,
        "client_waits": client_waits,
        "by_partition": by_partition,
        "slow_by_partition": slow_by_partition,
        "by_pod": by_pod,
        "dup_ids": dup_ids,
        "online": online,
        "offline": offline,
        "payload_types": payload_types,
        "payload_states": payload_states,
        "payload_fqdns": payload_fqdns,
        "payload_ids": payload_ids,
        "sent_times": sent_times,
        "pickup_times": pickup_times,
        "received_times": received_times,
        "stale_sent": stale_sent,
        "hot_client": hot_client,
        "hot_partition": hot_partition,
    }


def fmt_dt(value: datetime | None) -> str:
    if value is None:
        return "-"
    return value.astimezone(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")


def fmt_seconds(value: float) -> str:
    if value != value:  # NaN
        return "-"
    if abs(value) >= 10:
        return f"{value:.1f}s"
    if abs(value) >= 1:
        return f"{value:.2f}s"
    return f"{value * 1000:.1f}ms"


def conclusion(result: dict[str, Any]) -> list[str]:
    lines: list[str] = []
    waits = result["waits"]
    slow = result["slow"]
    pickups = result["pickups"]
    if not pickups:
        lines.append(
            "No 'Read message off of kafka topic' lines. Re-run with that message "
            "(or merge a pickup dump with this client dump)."
        )
        if result["payloads"] or result["dup_ids"]:
            lines.append("Payload / duplicate-ignore lines are present; volume/wait still need pickup logs.")
        return lines

    slow_pct = 100.0 * len(slow) / len(waits) if waits else 0.0
    if slow_pct < 5:
        lines.append(
            f"Pickup wait looks healthy in this sample ({slow_pct:.1f}% > {SLO_SECONDS:.0f}s). "
            "If the alert fired, this dump is the wrong window or is size-capped past the spike."
        )
        return lines

    hot_client = result["hot_client"]
    hot_part = result["hot_partition"]
    client_n = result["slow_by_client"].get(hot_client, 0) if hot_client else 0
    lines.append(
        f"{slow_pct:.1f}% of sampled pickups waited > {SLO_SECONDS:.0f}s "
        f"(p50 {fmt_seconds(percentile(waits, 50))}, max {fmt_seconds(max(waits))})."
    )
    if hot_client:
        share = 100.0 * client_n / len(slow) if slow else 0.0
        lines.append(
            f"Slow pickups concentrate on client_id {hot_client} "
            f"({client_n}/{len(slow)} = {share:.0f}%)"
            + (f" partition {hot_part}." if hot_part is not None else ".")
        )

    if result["online"] or result["payload_states"]:
        state = result["payload_states"].most_common(1)[0] if result["payload_states"] else ("?", 0)
        ptype = result["payload_types"].most_common(1)[0] if result["payload_types"] else ("?", 0)
        lines.append(
            f"Those messages are {ptype[0]} / {state[0]} "
            f"(online handler {result['online']}, offline {result['offline']})."
        )

    dup_n = sum(result["dup_ids"].values())
    unique_payload_ids = len(set(result["payload_ids"]))
    if dup_n and result["online"]:
        lines.append(
            f"{dup_n} lines are '{DUP_MSG}'. "
            "Handler is discarding them after a DB lookup; MQTT still enqueued them."
        )
        if unique_payload_ids and unique_payload_ids == len(result["payload_ids"]):
            lines.append(
                "Protocol message_id is unique per payload, so this is the old-`sent` path, "
                "not a repeated MQTT packet."
            )

    if result["sent_times"] and result["stale_sent"] > 0.8 * len(result["sent_times"]):
        lines.append(
            f"Payload `sent` is stale vs pickup "
            f"({fmt_dt(min(result['sent_times']))} .. {fmt_dt(max(result['sent_times']))} "
            f"vs pickup {fmt_dt(min(result['pickup_times']))} .. {fmt_dt(max(result['pickup_times']))}). "
            "Client is replaying/republishing old connection-status envelopes."
        )

    fqdn = result["payload_fqdns"].most_common(1)[0][0] if result["payload_fqdns"] else None
    if fqdn:
        facts = next((p for p in result["payloads"] if p.get("fqdn") == fqdn), {})
        extra = []
        if facts.get("insights_id"):
            extra.append(f"insights_id {facts['insights_id']}")
        if facts.get("subscription_manager_id"):
            extra.append(f"subman {facts['subscription_manager_id']}")
        lines.append("Host: " + fqdn + ((" (" + ", ".join(extra) + ")") if extra else "") + ".")

    lines.append(
        "Next: inventory/customer for that host; rhc logs for a publish loop or reconnect flush. "
        "Other partitions can look fine the whole time — messages are keyed by client_id."
    )
    return lines


def render(result: dict[str, Any]) -> str:
    out: list[str] = []
    out.append("Cloud Connector Kafka wait diagnosis")
    out.append("=" * 40)

    out.append("\nInputs")
    for path, total, returned in result["file_totals"]:
        capped = "  ** SIZE CAPPED — dump is a tail, not the full window **" if returned < total else ""
        out.append(f"  {path}")
        out.append(f"    ES hits.total={total}  returned={returned}{capped}")
    out.append(f"  merged unique hits: {result['returned_hits']}")

    waits = result["waits"]
    pickups = result["pickups"]
    out.append("\nPickup wait ( @timestamp - date_received )")
    if not waits:
        out.append("  (no pickup lines)")
    else:
        p50, p95, p99 = percentile(waits, 50), percentile(waits, 95), percentile(waits, 99)
        slow_n = len(result["slow"])
        out.append(f"  n={len(waits)}  p50={fmt_seconds(p50)}  p95={fmt_seconds(p95)}  p99={fmt_seconds(p99)}  max={fmt_seconds(max(waits))}")
        out.append(
            f"  >{SLO_SECONDS:.0f}s: {slow_n} ({100.0 * slow_n / len(waits):.1f}%)"
        )
        if result["pickup_times"]:
            out.append(
                f"  pickup window: {fmt_dt(min(result['pickup_times']))} -> {fmt_dt(max(result['pickup_times']))}"
            )
        if result["received_times"]:
            out.append(
                f"  date_received:  {fmt_dt(min(result['received_times']))} -> {fmt_dt(max(result['received_times']))}"
            )

    def top_table(counter: Counter, title: str, waits_for: dict | None = None, limit: int = 8) -> None:
        out.append(f"\n{title}")
        if not counter:
            out.append("  (none)")
            return
        for key, count in counter.most_common(limit):
            extra = ""
            if waits_for and key in waits_for:
                cw = sorted(waits_for[key])
                extra = f"  p50={fmt_seconds(percentile(cw, 50))}  max={fmt_seconds(max(cw))}  >1s={sum(1 for v in cw if v > SLO_SECONDS)}"
            out.append(f"  {count:6}  {key}{extra}")

    top_table(result["slow_by_client"] or result["by_client"], "Top client_id (slow pickups, else all pickups)", result["client_waits"])
    top_table(result["slow_by_partition"] or result["by_partition"], "Top partition")
    top_table(result["by_pod"], "Pods")

    out.append("\nLog line mix")
    for msg, count in result["messages"].most_common(12):
        out.append(f"  {count:6}  {msg}")

    out.append("\nPayload / handler")
    out.append(f"  online handler: {result['online']}   offline: {result['offline']}   dup/old ignore: {sum(result['dup_ids'].values())}")
    if result["payloads"]:
        out.append(f"  parsed payloads: {len(result['payloads'])}  unique message_id: {len(set(result['payload_ids']))}")
        if result["payload_types"]:
            out.append("  type: " + ", ".join(f"{k}={v}" for k, v in result["payload_types"].most_common()))
        if result["payload_states"]:
            out.append("  state: " + ", ".join(f"{k}={v}" for k, v in result["payload_states"].most_common()))
        if result["payload_fqdns"]:
            out.append("  fqdn: " + ", ".join(f"{k}={v}" for k, v in result["payload_fqdns"].most_common(5)))
        if result["sent_times"]:
            out.append(
                f"  payload sent: {fmt_dt(min(result['sent_times']))} -> {fmt_dt(max(result['sent_times']))}  "
                f"stale(>1h vs last pickup): {result['stale_sent']}/{len(result['sent_times'])}"
            )
        sample = next((p for p in result["payloads"] if p.get("insights_id") or p.get("fqdn")), None)
        if sample:
            out.append(f"  insights_id: {sample.get('insights_id')}")
            out.append(f"  subscription_manager_id: {sample.get('subscription_manager_id')}")
    else:
        out.append("  (no 'Received control message' payloads in these dumps)")

    out.append("\nConclusion")
    for line in conclusion(result):
        out.append(f"  {line}")
    return "\n".join(out) + "\n"


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("dumps", nargs="+", type=Path, help="ES _search JSON files")
    args = parser.parse_args(argv)

    dump = Dump()
    try:
        for path in args.dumps:
            dump.add_file(path)
    except (OSError, ValueError, json.JSONDecodeError) as err:
        print(f"error: {err}", file=sys.stderr)
        return 1

    if not dump.hits:
        print("error: no hits in dumps", file=sys.stderr)
        return 1

    sys.stdout.write(render(analyze(dump)))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
