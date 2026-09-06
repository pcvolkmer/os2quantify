ifndef VERBOSE
.SILENT:
endif

GITTAG = $(shell git describe --tag --abbrev=0 2>/dev/null | sed -En 's/v(.*)$$/\1/p')
ifeq ($(findstring -, $(GITTAG)), -)
    GITDEV = $(shell git describe --tag 2>/dev/null | sed -En 's/v(.*)-([0-9]+)-g([0-9a-f]+)$$/.dev.\2+\3/p')
else
    GITDEV = $(shell git describe --tag 2>/dev/null | sed -En 's/v(.*)-([0-9]+)-g([0-9a-f]+)$$/-dev.\2+\3/p')
endif
VERSION := "$(GITTAG)$(GITDEV)"

package-all: win-package linux-package

.PHONY: win-package
win-package: win-binary-x86_64
	mkdir -p os2quantify
	cp target/os2quantify.exe os2quantify/
	cp README.md os2quantify/
	cp LICENSE os2quantify/
	zip os2quantify-$(VERSION)_win64.zip os2quantify/* >/dev/null
	rm -rf os2quantify || true

.PHONY: linux-package
linux-package: linux-binary-x86_64
	mkdir -p os2quantify
	cp target/os2quantify os2quantify/
	cp README.md os2quantify/
	cp LICENSE os2quantify/
	tar -czvf os2quantify-$(VERSION)_linux.tar.gz os2quantify/ >/dev/null
	rm -rf os2quantify || true

binary-all: win-binary-x86_64 linux-binary-x86_64

.PHONY: win-binary-x86_64
win-binary-x86_64:
	mkdir -p target
	GOOS=windows GOARCH=amd64 go build -o target/os2quantify.exe -ldflags '-w -s' .

.PHONY: linux-binary-x86_64
linux-binary-x86_64:
	mkdir -p target
	GOOS=linux GOARCH=amd64 go build -o target/os2quantify -ldflags '-w -s' .

.PHONY: clean
clean:
	rm -rf target || true
	rm -rf os2quantify 2>/dev/null || true
	rm *_win64.zip 2>/dev/null || true
	rm *_linux.tar.gz 2>/dev/null || true