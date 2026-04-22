BACKEND_IMAGE  := ghcr.io/alleroux/plotbunni-backend
FRONTEND_IMAGE := ghcr.io/alleroux/plotbunni
BACKEND_SHA    := $(shell git -C . rev-parse HEAD)
FRONTEND_SHA   := $(shell git -C ../plotbunni rev-parse HEAD 2>/dev/null || echo latest)

# SSH config — override via environment or a local .makerc file
-include .makerc
STAGE_SSH_HOST ?=
STAGE_SSH_USER ?= deploy
PROD_SSH_HOST  ?=
PROD_SSH_USER  ?= deploy

.PHONY: deploy-stage deploy-prod push-backend push-frontend push

push-backend:
	docker build -t $(BACKEND_IMAGE):$(BACKEND_SHA) -t $(BACKEND_IMAGE):latest .
	docker push $(BACKEND_IMAGE):$(BACKEND_SHA)
	docker push $(BACKEND_IMAGE):latest

push-frontend:
	docker build -t $(FRONTEND_IMAGE):$(FRONTEND_SHA) -t $(FRONTEND_IMAGE):latest \
		--build-arg VITE_API_URL= ../plotbunni
	docker push $(FRONTEND_IMAGE):$(FRONTEND_SHA)
	docker push $(FRONTEND_IMAGE):latest

push: push-backend push-frontend

deploy-stage: push
	STAGE_SSH_HOST=$(STAGE_SSH_HOST) STAGE_SSH_USER=$(STAGE_SSH_USER) \
		bash scripts/deploy.sh stage $(BACKEND_SHA) $(FRONTEND_SHA)

deploy-prod: push
	PROD_SSH_HOST=$(PROD_SSH_HOST) PROD_SSH_USER=$(PROD_SSH_USER) \
		bash scripts/deploy.sh prod $(BACKEND_SHA) $(FRONTEND_SHA)
