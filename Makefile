UNAME := $(shell uname)

test:
	cd wrapminer; go vet
	cd launcher; go vet

build: wrapminer/*.go launcher/*.go
	cd wrapminer; go build -o ../build/; cp ../build/dmo-wrapminer* ../localtest/
	cd launcher; go build -o ../build/; cp ../build/dmo-launcher* ../localtest/

release: build/* example_configs/* changelog.md README.md
ifeq ($(UNAME),MINGW64_NT-10.0-19042)
	zip -j release/windows/dmowrapminer.zip build/* example_configs/* changelog.md Readme.md
	cp build/dmo-wrapminer* release/windows/
endif
ifeq ($(UNAME), Linux)
	tar -czvf release/linux/dmo-wrapminer.tar.gz -C build . -C ../example_configs . -C ../ changelog.md README.md
	cp build/dmo-wrapminer* release/linux/
endif

