import time
import httpx
import os
from notifications import notifier

API_URL = os.getenv("API_URL", "http://api:8000")
API_KEY = os.getenv("API_SECRET_KEY") 
CHECK_INTERVAL = 30

server_state_cache = {}

HEADERS = {
    "x-api-key": API_KEY,
    "Content-Type": "application/json"
}

def get_targets():
    try:
        response = httpx.get(f"{API_URL}/targets/", headers=HEADERS) 
        if response.status_code == 200:
            return response.json()
    except Exception as e:
        print(f"❌ Error fetching targets: {e}", flush=True)
    return []

def monitor_system():
    print("Starting monitor cycle...", flush=True)

    targets = get_targets()
    if not targets:
        print(" No targets configured.", flush=True)
        return

    server_reports = []

    for target in targets:
        name = target["name"]
        url = target["url"]
        current_status = "up"
        
        try:
            print(f" Connecting to {url}...", flush=True)
            res = httpx.get(url, timeout=10.0, follow_redirects=True)
            if res.status_code != 200:
                current_status = "down"
                print(f"❌ {name}: Invalid status ({res.status_code})", flush=True)
            else:
                print(f" {name}: OK ({res.status_code})", flush=True)
        except Exception as e:
            current_status = "down"
            print(f"❌ {name}: CONNECTION ERROR - {e}", flush=True)

        previous_status = server_state_cache.get(name)

        if previous_status is None:
            server_state_cache[name] = current_status
        elif previous_status != current_status:
            print(f"⚠️ STATE CHANGE: {name} | {previous_status} -> {current_status}", flush=True)
            if current_status == "down":
                notifier.send_alert(name, current_status, url)
            server_state_cache[name] = current_status

        server_reports.append({
            "name": name,
            "status": current_status
        })

    if server_reports:
        try:
            httpx.post(
                f"{API_URL}/servers/report",
                json={"servers": server_reports},
                headers=HEADERS 
            )
            print("💾 Report saved to DB.", flush=True)
        except Exception as e:
            print(f"❌ Error saving report to API: {e}", flush=True)

if __name__ == "__main__":
    print("Monitor Worker V5 (Authenticated)", flush=True)
    time.sleep(5)
    while True:
        monitor_system()
        print(f"Sleeping {CHECK_INTERVAL} seconds...", flush=True)
        time.sleep(CHECK_INTERVAL)