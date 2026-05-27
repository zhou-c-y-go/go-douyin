@echo off
cd /d "%~dp0"
cd /d "D:\minio\bin"

echo 🚀 MinIO Object Storage Service is starting...
echo 🔗 Web Console: http://127.0.0.1:9000
echo ⚡ API Endpoint: http://127.0.0.1:9005
minio.exe server D:\minio\data --console-address "127.0.0.1:9005" --address "127.0.0.1:9000"
pause