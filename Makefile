.PHONY: build run-dbms run-client client dbms

BIN_DIR := bin
DBMS_BIN := $(BIN_DIR)/ii64_dbms
CLIENT_BIN := $(BIN_DIR)/client
DEFAULT_ADDR := 127.0.0.1
DEFAULT_PORT := 7770

# dbms 빌드 (bin/ii64_dbms)
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(DBMS_BIN) ./cmd/ii64_dbms
	go build -o $(CLIENT_BIN) ./cmd/client

# dbms Go 로 바로 실행 (포트 7770, 현재 디렉터리 데이터)
run-dbms: dbms
dbms:
	go run ./cmd/ii64_dbms

# client 실행 (디폴트 127.0.0.1:7770, stdin 입력 대기)
run-client: client
client:
	go run ./cmd/client $(DEFAULT_ADDR) $(DEFAULT_PORT)
