#!/bin/bash
set -e

BASE_URL="http://localhost:8080/api/v1"
FILE_PATH="../test.mp4"

echo "1. Logging in..."
LOGIN_RES=$(curl -s -X POST $BASE_URL/auth/login -H "Content-Type: application/json" -d '{"email":"admin@ismart.com","password":"admin123"}')
TOKEN=$(echo $LOGIN_RES | grep -o '"access_token":"[^"]*' | grep -o '[^"]*$')

if [ -z "$TOKEN" ]; then
  echo "Login failed!"
  exit 1
fi

echo "2. Creating course..."
COURSE_RES=$(curl -s -X POST $BASE_URL/courses -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"title":"Test Course", "description":"Testing video upload", "category":"Test", "level":"Beginner"}')
COURSE_ID=$(echo $COURSE_RES | grep -o '"id":"[^"]*' | head -n 1 | grep -o '[^"]*$')

echo "Course ID: $COURSE_ID"

echo "3. Creating module..."
MODULE_RES=$(curl -s -X POST $BASE_URL/courses/$COURSE_ID/modules -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"title":"Test Module", "description":"Test module description", "order_index":1}')
MODULE_ID=$(echo $MODULE_RES | grep -o '"id":"[^"]*' | head -n 1 | grep -o '[^"]*$')

echo "Module ID: $MODULE_ID"

echo "4. Uploading video..."
curl -s -X POST $BASE_URL/courses/$COURSE_ID/modules/$MODULE_ID/upload -H "Authorization: Bearer $TOKEN" -F "file=@$FILE_PATH"

echo -e "\n5. Polling for processing progress..."
while true; do
  PROGRESS_RES=$(curl -s -X GET $BASE_URL/courses/$COURSE_ID/modules/$MODULE_ID/progress -H "Authorization: Bearer $TOKEN")
  echo "Progress: $PROGRESS_RES"
  
  if echo "$PROGRESS_RES" | grep -q '"status":"ready"'; then
    echo "✅ Processing complete!"
    break
  fi
  sleep 2
done

echo "6. Checking module status..."
curl -s -X GET $BASE_URL/courses/$COURSE_ID/modules -H "Authorization: Bearer $TOKEN"
echo ""
