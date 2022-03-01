UNAME := $(shell uname)

build: wrapminer/*.go launcher/*.go
	cd wrapminer; go build -o ../build/; cp ../build/dmo-wrapminer* ../localtest/
	cd launcher; go build -o ../build/; cp ../build/dmo-launcher* ../localtest/

release: build/* example_configs/* changelog.md Readme.md
ifeq ($(UNAME),MINGW64_NT-10.0-19042)
	zip -j release/windows/dmowrapminer.zip build/dmo-wrapminer* example_configs/* changelog.md Readme.md
endif
ifeq ($(UNAME), Linux)
endif

