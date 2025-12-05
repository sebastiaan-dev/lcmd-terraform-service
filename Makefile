.PHONY: certs-all clean

CERTS_DIR := certs
CA_KEY := $(CERTS_DIR)/ca.key
CA_CERT := $(CERTS_DIR)/ca.crt
SERVER_KEY := $(CERTS_DIR)/server.key
SERVER_CSR := $(CERTS_DIR)/server.csr
SERVER_CERT := $(CERTS_DIR)/server.crt
SERVER_EXT := $(CERTS_DIR)/server.ext
CLIENT_KEY := $(CERTS_DIR)/client.key
CLIENT_CSR := $(CERTS_DIR)/client.csr
CLIENT_CERT := $(CERTS_DIR)/client.crt
CLIENT_P12 := $(CERTS_DIR)/client.p12
CLIENT_PFX_PASSWORD ?= changeme

certs-all:	$(SERVER_CERT) $(CLIENT_CERT)

$(CERTS_DIR):
	mkdir -p $(CERTS_DIR)

$(CA_KEY): | $(CERTS_DIR)
	openssl genrsa -out $(CA_KEY) 4096

$(CA_CERT): $(CA_KEY)
	openssl req -x509 -new -nodes -key $(CA_KEY) -sha256 -days 3650 -out $(CA_CERT) -subj "/CN=lzc-local-ca"

$(SERVER_KEY): | $(CERTS_DIR)
	openssl genrsa -out $(SERVER_KEY) 2048

$(SERVER_CSR): $(SERVER_KEY)
	openssl req -new -key $(SERVER_KEY) -out $(SERVER_CSR) -subj "/CN=lzc-local-api"

$(SERVER_EXT): | $(CERTS_DIR)
	printf "subjectAltName=DNS:lzc-local-api,DNS:localhost\n" > $(SERVER_EXT)

$(SERVER_CERT): $(SERVER_CSR) $(CA_CERT) $(SERVER_EXT)
	openssl x509 -req -in $(SERVER_CSR) -CA $(CA_CERT) -CAkey $(CA_KEY) -CAcreateserial -out $(SERVER_CERT) -days 825 -sha256 -extfile $(SERVER_EXT)

$(CLIENT_KEY): | $(CERTS_DIR)
	openssl genrsa -out $(CLIENT_KEY) 2048

$(CLIENT_CSR): $(CLIENT_KEY)
	openssl req -new -key $(CLIENT_KEY) -out $(CLIENT_CSR) -subj "/CN=lzc-api-client"

$(CLIENT_CERT): $(CLIENT_CSR) $(CA_CERT)
	openssl x509 -req -in $(CLIENT_CSR) -CA $(CA_CERT) -CAkey $(CA_KEY) -CAcreateserial -out $(CLIENT_CERT) -days 825 -sha256
	openssl pkcs12 -export -clcerts -inkey $(CLIENT_KEY) -in $(CLIENT_CERT) -out $(CLIENT_P12) -passout pass:$(CLIENT_PFX_PASSWORD)
