.PHONY: build up attack traffic capture clean logs health

# Build all Docker images
build:
	docker compose build

# Start the 5G core network (NRF, UDM, AUSF, AMF, SMF)
up:
	docker compose up --build -d
	@echo ""
	@echo "╔══════════════════════════════════════════════╗"
	@echo "║  5G Core Network is starting...             ║"
	@echo "║  NRF:  http://localhost:8000/health          ║"
	@echo "║  UDM:  http://localhost:8001/health          ║"
	@echo "║  AUSF: http://localhost:8002/health          ║"
	@echo "║  AMF:  http://localhost:8003/health          ║"
	@echo "║  SMF:  http://localhost:8004/health          ║"
	@echo "╚══════════════════════════════════════════════╝"

# Run the attacker (cross-service token finding attack)
attack:
	docker compose --profile attack run --rm attacker

# Generate legitimate UE traffic
traffic:
	cd traffic && pip install -r requirements.txt -q && python3 legit_traffic.py

# Watch token stores in real-time
capture:
	cd traffic && pip install -r requirements.txt -q && python3 capture.py

# View logs from all services
logs:
	docker compose logs -f

# Check health of all services
health:
	@echo "Checking NRF..."  && curl -s http://localhost:8000/health | python3 -m json.tool 2>/dev/null || echo "NRF not responding"
	@echo "Checking UDM..."  && curl -s http://localhost:8001/health | python3 -m json.tool 2>/dev/null || echo "UDM not responding"
	@echo "Checking AUSF..." && curl -s http://localhost:8002/health | python3 -m json.tool 2>/dev/null || echo "AUSF not responding"
	@echo "Checking AMF..."  && curl -s http://localhost:8003/health | python3 -m json.tool 2>/dev/null || echo "AMF not responding"
	@echo "Checking SMF..."  && curl -s http://localhost:8004/health | python3 -m json.tool 2>/dev/null || echo "SMF not responding"

# Stop and clean up everything
clean:
	docker compose --profile attack down -v --rmi local
	@echo "Cleaned up all containers, volumes, and images."
