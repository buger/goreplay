web: python -m SimpleHTTPServer 8000
replayed_web: python -m SimpleHTTPServer 8001
listener: sudo -E go run gor.go listen -p 8000 -r localhost:8002 --verbose
replay: go run gor.go replay -f localhost:8001 -p 8002 --verbose
