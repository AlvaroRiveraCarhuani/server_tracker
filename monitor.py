import time
import httpx
import os

API_URL = os.getenv("API_URL", "http://api:8000") 
CHECK_INTERVAL = 30  

def get_targets():
    try:
        response = httpx.get(f"{API_URL}/targets/")
        if response.status_code == 200:
            return response.json()
    except Exception as e:
        print(f" Error fetching targets: {e}")
    return []

def monitor_system():
    print(" Starting monitor cycle...")
    
    targets = get_targets()
    if not targets:
        print("No targets configured in database.")
        return

    server_reports = []

    for target in targets:
        name = target['name']
        url = target['url']
        status = "up"
        
        try:
            res = httpx.get(url, timeout=5.0)
            if res.status_code != 200:
                status = "down"
                print(f"🔻 {name} returned status {res.status_code}")
        except Exception as e:
            status = "down"
            print(f"🔻 {name} is unreachable: {e}")

        server_reports.append({
            "name": name,
            "status": status
        })

    if server_reports:
        try:
            payload = {"servers": server_reports}
            httpx.post(f"{API_URL}/servers/report", json=payload)
            print(" Report sent to API.")
        except Exception as e:
            print(f" Error sending report to API: {e}")

if __name__ == "__main__":
    print(" Monitor Worker Started (English Version)")
    # Wait for API to boot up
    time.sleep(5)
    
    while True:
        monitor_system()
        print(f"Waiting {CHECK_INTERVAL} seconds...")
        time.sleep(CHECK_INTERVAL)