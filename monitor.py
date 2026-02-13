import time
import httpx
import os
# Importamos nuestro nuevo sistema modular
from notifications import notifier

API_URL = os.getenv("API_URL", "http://api:8000")
CHECK_INTERVAL = 30

# MEMORIA RAM: Guarda el estado anterior para detectar cambios
# Estructura: { "Google": "up", "Facebook": "down" }
server_state_cache = {}

def get_targets():
    """Obtiene la lista de servidores a monitorear desde la API"""
    try:
        response = httpx.get(f"{API_URL}/targets/")
        if response.status_code == 200:
            return response.json()
    except Exception as e:
        print(f"❌ Error conectando con API para targets: {e}", flush=True)
    return []

def monitor_system():
    print("🔍 Iniciando ciclo de monitoreo...", flush=True)

    targets = get_targets()
    if not targets:
        print("📭 No hay objetivos configurados.", flush=True)
        return

    server_reports = []

    for target in targets:
        name = target["name"]
        url = target["url"]

        # 1. DETERMINAR ESTADO ACTUAL
        current_status = "up" # Asumimos UP hasta que se demuestre lo contrario
        
        try:
            print(f"📡 Conectando a {url}...", flush=True)
            # follow_redirects=True evita falsos positivos con 301/302
            res = httpx.get(url, timeout=10.0, follow_redirects=True)

            if res.status_code != 200:
                current_status = "down"
                print(f"❌ {name}: Status incorrecto ({res.status_code})", flush=True)
            else:
                print(f"✅ {name}: OK ({res.status_code})", flush=True)

        except Exception as e:
            current_status = "down"
            print(f"❌ {name}: ERROR DE CONEXIÓN - {e}", flush=True)

        # 2. LÓGICA DE NOTIFICACIONES E INTELIGENCIA
        previous_status = server_state_cache.get(name)

        # --- TRUCO DE TESTING ---
        # Si es la primera vez que lo vemos, fingimos que antes estaba "up".
        # Así, si ahora está "down", detectará un CAMBIO y enviará alerta inmediata.
        if previous_status is None:
            previous_status = "up" 

        # Si el estado CAMBIÓ respecto a la memoria
        if previous_status != current_status:
            print(f"⚠️ CAMBIO DE ESTADO: {name} | Antes: {previous_status} -> Ahora: {current_status}", flush=True)

            # Solo notificamos si se CAYÓ (puedes quitar el if si quieres notificar también cuando vuelve)
            if current_status == "down":
                # Usamos el sistema modular (No importa si es Discord o Telegram)
                notifier.send_alert(name, current_status, url)

            # ACTUALIZAMOS LA MEMORIA RAM
            server_state_cache[name] = current_status

        # Agregamos al reporte para la base de datos
        server_reports.append({
            "name": name,
            "status": current_status
        })

    # 3. ENVIAR REPORTE A LA API (Para Grafana y Logs)
    if server_reports:
        try:
            httpx.post(
                f"{API_URL}/servers/report",
                json={"servers": server_reports}
            )
            print("💾 Reporte guardado en Base de Datos.", flush=True)
        except Exception as e:
            print(f"❌ Error guardando reporte en API: {e}", flush=True)

if __name__ == "__main__":
    print("🤖 Monitor Worker V4 (Modular Notifications Started)", flush=True)
    # Esperamos un poco a que la API arranque
    time.sleep(5)

    while True:
        monitor_system()
        print(f"💤 Durmiendo {CHECK_INTERVAL} segundos...", flush=True)
        time.sleep(CHECK_INTERVAL)