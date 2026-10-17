FUZZTIME ?= 60s

.PHONY: test gametest fuzz vet fmt cover

## test: run every test that needs no game installation
test:
	go test ./...

## gametest: also run the tests against a real installation, for example
##   make gametest PDX_GAME_DIR="C:/Steam/steamapps/common/Victoria 3/game"
## PDX_MOD_DIRS optionally adds mods on top, in load order, separated by ; on
## Windows and : elsewhere.
gametest:
	PDX_GAME_DIR="$(PDX_GAME_DIR)" PDX_MOD_DIRS="$(PDX_MOD_DIRS)" go test -count=1 -v ./...

## fuzz: fuzz every parser for FUZZTIME each
fuzz:
	go test ./script -run '^$$' -fuzz FuzzParse -fuzztime $(FUZZTIME)
	go test ./localization -run '^$$' -fuzz FuzzParse -fuzztime $(FUZZTIME)
	go test ./victoria3 -run '^$$' -fuzz FuzzDecode -fuzztime $(FUZZTIME)
	go test ./asset -run '^$$' -fuzz FuzzRead -fuzztime $(FUZZTIME)
	go test ./genes -run '^$$' -fuzz FuzzRead -fuzztime $(FUZZTIME)
	go test ./portrait -run '^$$' -fuzz FuzzRead -fuzztime $(FUZZTIME)

## cover: show test coverage by package
cover:
	go test -cover ./...

vet:
	go vet ./...

fmt:
	go fmt ./...
