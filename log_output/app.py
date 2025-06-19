import datetime
import uuid
import time

if __name__ == "__main__":    
    myuuid = str(uuid.uuid4()) # Generate a unique identifier
    
    while True:
        # Get the current date and time
        now = datetime.datetime.now(datetime.timezone.utc)
        timestamp = now.isoformat(timespec='milliseconds')
        print(timestamp + ": " + myuuid)
        # Sleep for 5 seconds
        time.sleep(5)
