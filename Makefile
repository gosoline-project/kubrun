# Simple Docker build & push helpers (GitHub Container Registry)

# Set GH_OWNER to your GitHub username or org (without ghcr.io/)
GH_OWNER?=gosoline-project
IMAGE?=ghcr.io/$(GH_OWNER)/kubrun
TAG?=latest
# Optional extra version tag (e.g. make push VERSION=0.1.0)
VERSION?=

.PHONY: build push login release

# Login expects a GHCR PAT with write:packages scope in GITHUB_TOKEN or GHCR_TOKEN
login:
	@token=$${GHCR_TOKEN:-$${GITHUB_TOKEN:-}}; \
	if [ -z "$$token" ]; then echo "Set GHCR_TOKEN or GITHUB_TOKEN"; exit 1; fi; \
	echo "$$token" | docker login ghcr.io -u $(GH_OWNER) --password-stdin

build:
	docker build -t $(IMAGE):$(TAG) $(if $(VERSION),-t $(IMAGE):$(VERSION),) .

push: build
	docker push $(IMAGE):$(TAG)
	@if [ -n "$(VERSION)" ]; then docker push $(IMAGE):$(VERSION); fi

release:
	@if [ -z "$(VERSION)" ]; then echo "Set VERSION=X.Y.Z"; exit 1; fi
	$(MAKE) push VERSION=$(VERSION) TAG=latest
