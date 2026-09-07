📡 Server Tracker

A robust, containerized uptime monitoring solution built with Python (FastAPI), PostgreSQL, and Grafana.

📖 Overview

Server Tracker is a microservices-based application designed to monitor the health and uptime of websites and servers.

It performs periodic checks, logs historical data into a PostgreSQL database, visualizes metrics via Grafana, and sends real-time alerts to communication channels (like Discord) when a service goes down.

Unlike simple scripts, this project uses the Strategy Pattern for notifications, ensuring the system is modular and scalable for future integrations (Telegram, Slack, Email).

🚀 Key Features

🧱 Microservices Architecture – Separated API, Database, and Background Worker

⏱️ Real-time Monitoring – Checks server status (HTTP/HTTPS) every 30 seconds

🚨 Smart Alerts – Prevents notification spam using an in-memory state cache (only alerts on status change)

📢 Multi-Channel Notifications – Modular system currently supporting Discord Webhooks

📊 Data Visualization – Integrated Grafana dashboards for uptime history and latency

🔄 Smart Redirection – Automatically handles 301/302 redirects to avoid false positives

🐳 Containerized – Fully deployable via Docker Compose

🛠️ Tech Stack

Backend API: FastAPI (Python)

Worker: Python Script (httpx + schedule)

Database: PostgreSQL 15

Visualization: Grafana

Containerization: Docker & Docker Compose

Database Management: Adminer (Lightweight UI)

📂 Project Structure
server_tracker/
├── main.py              # API Entry point
├── monitor.py           # Background Worker (The "Brain")
├── docker-compose.yml   # Orchestration of 4 services
├── notifications/       # 📢 Modular Notification System
│   ├── __init__.py      # Manager (Singleton)
│   ├── base.py          # Abstract Base Class (Interface)
│   └── discord.py       # Discord Implementation
├── routers/             # API Endpoints
├── models/              # Database Models (SQLAlchemy)
└── schemas/             # Pydantic Schemas

⚡ Getting Started
Prerequisites

Docker

Docker Compose

Git

Installation
1️⃣ Clone the repository
git clone https://github.com/AlvaroRiveraCarhuani/server_tracker.git
cd server_tracker

2️⃣ Configure Environment Variables

Create a .env file in the root directory:

# Database Config
POSTGRES_USER=tu_usuario
POSTGRES_PASSWORD=tu_contraseña
POSTGRES_DB=server_tracker_db
DATABASE_URL=postgresql://tu_usuario:tu_contraseña@db:5432/server_tracker_db

# Internal API Communication
API_URL=http://api:8000

# Notifications (Optional)
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/your_webhook_here

3️⃣ Launch the System
docker compose up --build

🖥️ Usage

Once the containers are running, you can access the services:

Service	URL	Description	Credentials (Default)
API Docs	http://localhost:8000/docs
	Swagger UI to manage targets	N/A
Grafana	http://localhost:3000
	Visualization Dashboards	admin / admin
Adminer	http://localhost:8080
	Database GUI	User/Pass from .env
➕ How to Add a Server to Monitor

Go to:

http://localhost:8000/docs


Use the POST /targets/ endpoint

Example payload:

{
  "name": "Google Production",
  "url": "https://google.com"
}


The Monitor Worker will automatically pick up the new target in the next cycle (30 seconds).

📊 Monitoring & Alerts
📈 Grafana

Connect Grafana to PostgreSQL

Host: db

User: postgres (or your configured user)

Create dashboards to visualize uptime logs and latency

📢 Discord Alerts

If a server returns:

Non-200 status code (e.g., 500)

Connection error

Timeout

An alert will be sent to your configured Discord channel.

🗺️ Roadmap

✅ Phase 1: Core API & Database

✅ Phase 2: Docker Orchestration & Grafana

✅ Phase 3: Modular Notification System (Discord)

🔜 Phase 4: Deployment to AWS (EC2)

🔐 Phase 5: Authentication (JWT) & Security

🖥️ Phase 6: Frontend Web Interface (React/Streamlit)

🤝 Contributing

This is an open-source educational project.

Pull requests are welcome to add new notification providers (Telegram, Slack, Email) inside the notifications/ folder.

Steps:

Fork the project

Create your feature branch

git checkout -b feature/AmazingFeature


Commit your changes

git commit -m "Add some AmazingFeature"


Push to the branch

git push origin feature/AmazingFeature


Open a Pull Request