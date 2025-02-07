import json
import os
import logging
import requests
from datetime import datetime, timedelta
import psycopg2
from psycopg2.extras import RealDictCursor
from dotenv import load_dotenv
import os

load_dotenv()

history_ids = {
    1: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6ImpkenZ6cXZrIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOlt7InZhcmlhYmxlSWQiOiI2OGttZXJrbCIsInZhbHVlSWRzIjpbIjRxeTRqMjZxIl19XSwidmlkZW8iOjB9LCJwYWdlIjoxLCJ2YXJ5IjoxNzM3MTYxNDYxfQ",
    2: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6IjgyNHI0eWdkIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOltdLCJ2aWRlbyI6MH0sInBhZ2UiOjEsInZhcnkiOjE3MzcxNjE0NjF9",
    3: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6IjlkOGc5NzNrIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOltdLCJ2aWRlbyI6MH0sInBhZ2UiOjEsInZhcnkiOjE3MzcxNjE0NjF9",
    4: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6IjAycWx6cXBrIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOlt7InZhcmlhYmxlSWQiOiJqODRrbTN3biIsInZhbHVlSWRzIjpbIjlxamR4azdxIl19XSwidmlkZW8iOjB9LCJwYWdlIjoxLCJ2YXJ5IjoxNzM3MTYxNDYxfQ",
    5: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6Im1rZXJucm5kIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOlt7InZhcmlhYmxlSWQiOiI1bHk3anBnbCIsInZhbHVlSWRzIjpbIm1sbjMyZW5xIl19XSwidmlkZW8iOjB9LCJwYWdlIjoxLCJ2YXJ5IjoxNzM3MTYxNDYxfQ",
    6: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6IjgyNDFlbHcyIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOltdLCJ2aWRlbyI6MH0sInBhZ2UiOjEsInZhcnkiOjE3MzcxNjE0NjF9",
    7: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6IjdkZ25nODcyIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOlt7InZhcmlhYmxlSWQiOiJ3bDNkM2d5OCIsInZhbHVlSWRzIjpbIjRseG4zMDQxIl19XSwidmlkZW8iOjB9LCJwYWdlIjoxLCJ2YXJ5IjoxNzM3MTYxNDYxfQ",
    8: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6InpkM295bW5kIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOlt7InZhcmlhYmxlSWQiOiI3ODlkajU5biIsInZhbHVlSWRzIjpbInpxbzRkbXgxIl19XSwidmlkZW8iOjB9LCJwYWdlIjoxLCJ2YXJ5IjoxNzM3MTYxNDYxfQ",
    9: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6InEyNXg1OHZrIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOlt7InZhcmlhYmxlSWQiOiJlOG1xcm13biIsInZhbHVlSWRzIjpbImpxemo3ZW1sIl19XSwidmlkZW8iOjB9LCJwYWdlIjoxLCJ2YXJ5IjoxNzM3MTYxNDYxfQ",
    10: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6IjdrajkwOW4yIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOlt7InZhcmlhYmxlSWQiOiJnbngyeW80OCIsInZhbHVlSWRzIjpbInE3NXZyb3IxIl19XSwidmlkZW8iOjB9LCJwYWdlIjoxLCJ2YXJ5IjoxNzM3MTYxNDYxfQ",
    11: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6IjlrdmxwOTAyIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOlt7InZhcmlhYmxlSWQiOiI5bDc1b2R6OCIsInZhbHVlSWRzIjpbIjE5MmpvZWtxIl19XSwidmlkZW8iOjB9LCJwYWdlIjoxLCJ2YXJ5IjoxNzM3MTYxNDYxfQ",
    12: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6IjlkODh4NmxkIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOlt7InZhcmlhYmxlSWQiOiJqbHp4dno3OCIsInZhbHVlSWRzIjpbImx4NXY3MnIxIl19XSwidmlkZW8iOjB9LCJwYWdlIjoxLCJ2YXJ5IjoxNzM3MTYxNDYxfQ",
    13: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6IndkbTFwbTNkIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOlt7InZhcmlhYmxlSWQiOiJvbnY5MTM3OCIsInZhbHVlSWRzIjpbIjFweTRueGcxIl19XSwidmlkZW8iOjB9LCJwYWdlIjoxLCJ2YXJ5IjoxNzM3MTYxNDYxfQ",
    14: "eyJwYXJhbXMiOnsiY2F0ZWdvcnlJZCI6IjlrdnpvcDhkIiwiZW11bGF0b3IiOjAsImdhbWVJZCI6IjRkN3k1emQ3Iiwib2Jzb2xldGUiOjAsInBsYXRmb3JtSWRzIjpbXSwicmVnaW9uSWRzIjpbXSwidGltZXIiOjAsInZlcmlmaWVkIjoxLCJ2YWx1ZXMiOltdLCJ2aWRlbyI6MH0sInBhZ2UiOjEsInZhcnkiOjE3MzcxNjE0NjF9",
}

# Mapping activity IDs to history IDs
def get_history_id(activity_id) -> str:
    id = history_ids.get(activity_id, None)

    if not id:
        raise ValueError(f"Unknown activity ID: {activity_id}")
    
    return id
        
# Store all speedrun data
def store_all_speedrun_data():
    all_data = []

    try:
        conn = psycopg2.connect(
            dbname=os.getenv("POSTGRES_DB"),
            user=os.getenv("POSTGRES_USER"),
            password=os.getenv("POSTGRES_PASSWORD"),
        )
        cur = conn.cursor(cursor_factory=RealDictCursor)

        cur.execute("SELECT id, name, release_date FROM activity_definition WHERE is_raid")
        activities = cur.fetchall()

        for activity in activities:
            activity_id = activity["id"]
            raid_name = activity["name"]
            date_released = activity["release_date"].replace(tzinfo=None)

            runs = fetch_speedrun_data(activity_id)
            output_runs = [
                {
                    "date": run["date"].strftime("%Y-%m-%d"),
                    "days_after_release": (run["date"] - date_released).days,
                    "time": run["time"],
                    "time_string": str(timedelta(seconds=run["time"]))
                }
                for run in runs
            ]

            all_data.append({
                "activity_id": activity_id,
                "raid_name": raid_name,
                "date_released": date_released.strftime("%Y-%m-%d"),
                "runs": output_runs
            })

        cur.close()
        conn.close()

        with open("speedrun_data.json", "w") as file:
            json.dump(all_data, file, indent=2)

        logging.info("Successfully stored speedrun data")
    except Exception as e:
        logging.error(f"Failed to store speedrun data: {e}")


# Fetch speedrun data
def fetch_speedrun_data(activity_id):
    history_id = get_history_id(activity_id)

    url = f"https://www.speedrun.com/api/v2/GetGameRecordHistory?_r={history_id}"
    try:
        response = requests.get(url)
        response.raise_for_status()
        data = response.json()
        run_list = data.get("runList", [])
        return [
            {
                "date": datetime.fromtimestamp(run["date"]).replace(tzinfo=None),
                "time": run["time"]
            }
            for run in run_list
        ]
    except requests.RequestException as e:
        logging.error(f"Failed to fetch speedrun data for activity {activity_id}: {e}")
        return []


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    store_all_speedrun_data()