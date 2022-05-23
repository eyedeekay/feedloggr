GO111MODULE=on
REPO_NAME=feedloggr
USER_GH=eyedeekay
VERSION=0.0.48
PWD=`pwd`

ARG=-v -tags netgo,osusergo -ldflags '-w -s -extldflags "-static"'
CGO=0

all: plugins winplugin linplugin

plugins: clean index

winplugin: plugins
	GOOS=windows GOARCH=amd64 make windows feedloggr-plugin

linplugin: plugins
	GOOS=linux GOARCH=amd64 make feedloggr feedloggr-plugin

clean:
	rm -frv proxy proxy.exe feedloggr feedloggr.exe feedloggr-windows feedloggr-windows.exe $(REPO_NAME) $(REPO_NAME).exe plugin feedloggr-zip feedloggr-zip-win *.su3 *.zip
	find . -name '*.go' -exec gofmt -w -s {} \;

feedloggr:
	go build $(ARG) -o feedloggr-$(GOOS) ./cmd/feedloggr

rb:
	/usr/lib/go-1.16/bin/go build $(ARG) -o feedloggr-$(GOOS) ./cmd/feedloggr

windows:
	GOOS=windows GOARCH=amd64 make feedloggr

SIGNER_DIR=$(HOME)/i2p-go-keys/

feedloggr-plugin: res
	i2p.plugin.native -name=feedloggr-$(GOOS) \
		-signer=hankhill19580@gmail.com \
		-signer-dir=$(SIGNER_DIR) \
		-version="$(VERSION)" \
		-author=hankhill19580@gmail.com \
		-autostart=true \
		-clientname=feedloggr-$(GOOS) \
		-consolename="feedloggr RSS feed" \
		-consoleurl="http://127.0.0.1:7681" \
		-icondata="icon/icon.png" \
		-delaystart="1" \
		-desc="`cat desc`" \
		-exename=feedloggr-$(GOOS) \
		-website="http://idk.i2p/feedloggr/" \
		-updateurl=http://idk.i2p/feedloggr/feedloggr-$(GOOS).su3 \
		-command="feedloggr-$(GOOS) -config \$$PLUGIN/anon-feedloggr.conf -dir \$$I2P/eepsite/docroot/rss" \
		-license=MIT \
		-res=tmp/
	unzip -o feedloggr-$(GOOS).zip -d feedloggr-$(GOOS)-zip

res:
	mkdir -p tmp
	cp anon-feedloggr.conf tmp/anon-feedloggr.conf
	cp LICENSE tmp/LICENSE

index:
	@echo "<!DOCTYPE html>" > index.html
	@echo "<html>" >> index.html
	@echo "<head>" >> index.html
	@echo "  <title>$(REPO_NAME)</title>" >> index.html
	@echo "  <link rel=\"stylesheet\" type=\"text/css\" href =\"home.css\" />" >> index.html
	@echo "</head>" >> index.html
	@echo "<body>" >> index.html
	markdown README.md | tee -a index.html
	@echo "</body>" >> index.html
	@echo "</html>" >> index.html

export sumsflinux=`sha256sum "./feedloggr-linux.su3"`
export sumsfwindows=`sha256sum "./feedloggr-windows.su3"`
export sumsflinuxbin=`sha256sum "./feedloggr-linux"`
export sumsfwindowsbin=`sha256sum "./feedloggr-windows.exe"`

release: all version upload-plugins

version:
	cat README.md | gothub release -s $(GITHUB_TOKEN) -u $(USER_GH) -r $(REPO_NAME) -t v$(VERSION) -d -; true

download-su3s:
	GOOS=windows GOARCH=amd64 make download-single-su3
	GOOS=linux GOARCH=amd64 make download-single-su3

download-single-su3:
	wget-ds "https://github.com/$(USER_GH)/$(REPO_NAME)/releases/download/v$(VERSION)/feedloggr-$(GOOS).su3"

upload-su3s: upload-plugins

upload-plugins:
	gothub upload -R -u $(USER_GH) -r "$(REPO_NAME)" -t v$(VERSION) -l "$(sumsflinux)" -n "feedloggr-linux.su3" -f "./feedloggr-linux.su3"
	gothub upload -R -u $(USER_GH) -r "$(REPO_NAME)" -t v$(VERSION) -l "$(sumsfwindows)" -n "feedloggr-windows.su3" -f "./feedloggr-windows.su3"
	gothub upload -R -u $(USER_GH) -r "$(REPO_NAME)" -t v$(VERSION) -l "$(sumsfwindowsbin)" -n "feedloggr-windows.exe" -f "./feedloggr-windows.exe"
	gothub upload -R -u $(USER_GH) -r "$(REPO_NAME)" -t v$(VERSION) -l "$(sumsflinuxbin)" -n "feedloggr-linux" -f "./feedloggr-windows"
