SHELL:=/bin/bash
GITCOMMIT := $(shell git describe --always)
GITCOMMITDATE := $(shell git log -1 --date=short --pretty=format:%cd)
GITBRANCH := $(CI_COMMIT_BRANCH)
BUILDDATE := $(shell TZ=UTC date +%Y-%m-%dT%H:%M:%SZ)
VERSION := "v4.1.2"
PROXY_EXISTS := $(shell if [[ "${https_proxy}" || "${http_proxy}" ]]; then echo 1; else echo 0; fi)
DOCKER_PROXY_FLAGS := ""
ifeq ($(PROXY_EXISTS),1)
	DOCKER_PROXY_FLAGS = --build-arg http_proxy=${http_proxy} --build-arg https_proxy=${https_proxy}
else
	undefine DOCKER_PROXY_FLAGS
endif
MONOREPO_GITURL := "https://github.com/intel-secl/intel-secl"
#TODO use the latest tag
MONOREPO_GITBRANCH := "v4.1.2"

gta:
	env GOOS=linux GOSUMDB=off GOPROXY=direct go mod tidy && env CGO_CFLAGS_ALLOW="-f.*" GOOS=linux GOSUMDB=off GOPROXY=direct go build -ldflags "-X intel/isecl/go-trust-agent/v4/util.Branch=$(GITBRANCH) -X intel/isecl/go-trust-agent/v4/util.Version=$(VERSION) -X intel/isecl/go-trust-agent/v4/util.GitHash=$(GITCOMMIT) -X intel/isecl/go-trust-agent/v4/util.BuildDate=$(BUILDDATE)" -o out/tagent

swagger-get:
	wget https://github.com/go-swagger/go-swagger/releases/download/v0.21.0/swagger_linux_amd64 -O /usr/local/bin/swagger
	chmod +x /usr/local/bin/swagger
	wget https://repo1.maven.org/maven2/io/swagger/codegen/v3/swagger-codegen-cli/3.0.16/swagger-codegen-cli-3.0.16.jar -O /usr/local/bin/swagger-codegen-cli.jar

swagger-doc:
	mkdir -p out/swagger
	env GOOS=linux GOSUMDB=off GOPROXY=direct go mod tidy
	export CGO_CFLAGS_ALLOW="-f.*"; /usr/local/bin/swagger generate spec -o ./out/swagger/openapi.yml --scan-models
	java -jar /usr/local/bin/swagger-codegen-cli.jar generate -i ./out/swagger/openapi.yml -o ./out/swagger/ -l html2 -t ./swagger/templates/

swagger: swagger-get swagger-doc

installer: gta download_upgrade_scripts
	mkdir -p out/installer
	cp dist/linux/tagent.service out/installer/tagent.service
	cp dist/linux/tagent_init.service out/installer/tagent_init.service
	cp dist/linux/install.sh out/installer/install.sh && chmod +x out/installer/install.sh
	cp dist/linux/manifest_tpm20.xml out/installer/manifest_tpm20.xml
	cp dist/linux/manifest_wlagent.xml out/installer/manifest_wlagent.xml

	cd tboot-xm && $(MAKE) package
	cp tboot-xm/out/application-agent*.bin out/installer/

	cp -a out/upgrades/* out/installer/
	cp -a upgrades/* out/installer/
	mv out/installer/build/* out/installer/
	chmod +x out/installer/*.sh

	cp out/tagent out/installer/tagent
	makeself out/installer out/trustagent-$(VERSION).bin "TrustAgent $(VERSION)" ./install.sh

download_upgrade_scripts: 
	git clone --depth 1 -b $(MONOREPO_GITBRANCH) $(MONOREPO_GITURL) monorepo_tmp
	cp -a monorepo_tmp/pkg/lib/common/upgrades out/
	chmod +x out/upgrades/*.sh
	rm -rf monorepo_tmp

unit_test_bin:
	env GOOS=linux GOSUMDB=off GOPROXY=direct go mod tidy && env CGO_CFLAGS_ALLOW="-f.*" GOOS=linux GOSUMDB=off GOPROXY=direct go build -tags=unit_test -gcflags=all="-N -l" -ldflags "-X intel/isecl/go-trust-agent/v4/util.Branch=$(GITBRANCH) -X intel/isecl/go-trust-agent/v4/util.Version=$(VERSION) -X intel/isecl/go-trust-agent/v4/util.GitHash=$(GITCOMMIT) -X intel/isecl/go-trust-agent/v4/util.BuildDate=$(BUILDDATE)" -o out/tagent

unit_test: unit_test_bin
	mkdir -p out
	env GOOS=linux GOSUMDB=off GOPROXY=direct go mod tidy
	env CGO_CFLAGS_ALLOW="-f.*" GOOS=linux GOSUMDB=off GOPROXY=direct go test ./... -tags=unit_test -coverpkg=./... -coverprofile out/cover.out
	go tool cover -func out/cover.out
	go tool cover -html=out/cover.out -o out/cover.html

oci-archive: gta download_upgrade_scripts
	docker build ${DOCKER_PROXY_FLAGS} -t isecl/tagent:$(VERSION) --label org.label-schema.build-date=$(BUILDDATE) -f dist/docker/Dockerfile .
	skopeo copy docker-daemon:isecl/tagent:$(VERSION) oci-archive:out/tagent-$(VERSION)-$(GITCOMMIT).tar

k8s: oci-archive
	cp -r dist/k8s out/

all: clean installer

clean:
	cd tboot-xm && $(MAKE) clean
	rm -rf out/
	rm -rf monorepo_tmp
