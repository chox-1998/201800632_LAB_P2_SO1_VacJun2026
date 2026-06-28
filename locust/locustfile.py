from locust import HttpUser, task, between
import random
from datetime import datetime

TEAMS = [
    "GTM",
    "MEX",
    "BRA",
    "ARG",
    "ESP"
]

class PredictionUser(HttpUser):

    wait_time = between(0.2, 1)

    @task
    def send_prediction(self):

        home = random.choice(TEAMS)
        away = random.choice([t for t in TEAMS if t != home])

        payload = {
            "home_team": home,
            "away_team": away,
            "home_goals": random.randint(0, 5),
            "away_goals": random.randint(0, 5),
            "username": f"user_{random.randint(1,1000)}",
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }

        self.client.post(
            "/grpc-202012345",
            json=payload
        )
