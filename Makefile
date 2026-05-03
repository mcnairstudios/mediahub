BINARY = mediahub
PID_FILE = /tmp/mediahub.pid
LOG_FILE = /tmp/mediahub.log
DATA_DIR = /tmp/mediahub-data
RECORD_DIR = /tmp/mediahub-recordings
LISTEN_ADDR = :9090
BASE_URL = http://192.168.0.111
USER_AGENT = Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36

.PHONY: build stop start restart test smoke clean

build:
	CGO_ENABLED=1 go build -o ./$(BINARY) ./cmd/mediahub/

stop:
	@if [ -f $(PID_FILE) ]; then kill -9 $$(cat $(PID_FILE)) 2>/dev/null || true; fi
	@pkill -9 -f "./$(BINARY)" 2>/dev/null || true
	@sleep 1
	@while pgrep -f "./$(BINARY)" >/dev/null 2>&1; do kill -9 $$(pgrep -f "./$(BINARY)") 2>/dev/null; sleep 1; done
	@rm -f $(PID_FILE)
	@echo "stopped"

start: stop build
	@mkdir -p $(DATA_DIR) $(RECORD_DIR)
	@rm -rf /private/var/folders/kr/*/T/mediahub-sessions/ 2>/dev/null || true
	@MEDIAHUB_DATA_DIR=$(DATA_DIR) \
	 MEDIAHUB_LISTEN_ADDR=$(LISTEN_ADDR) \
	 MEDIAHUB_USER_AGENT="$(USER_AGENT)" \
	 MEDIAHUB_RECORD_DIR=$(RECORD_DIR) \
	 MEDIAHUB_VOD_OUTPUT_DIR=$(RECORD_DIR) \
	 MEDIAHUB_BASE_URL=$(BASE_URL) \
	 nohup ./$(BINARY) > $(LOG_FILE) 2>&1 & echo $$! > $(PID_FILE)
	@sleep 3
	@curl -s -o /dev/null -w "http %{http_code}\n" http://localhost$(LISTEN_ADDR)/ 2>/dev/null || echo "not responding"
	@echo "started (pid $$(cat $(PID_FILE)), log $(LOG_FILE))"

restart: start

test:
	go test ./pkg/...

smoke:
	node web/dist/smoke_test.js

logs:
	@tail -f $(LOG_FILE)

clean:
	rm -f ./$(BINARY) $(PID_FILE)
