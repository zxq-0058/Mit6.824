RACE=-race
build:
# make sure software is freshly built.
	(cd ../../mrapps && go clean)
	(cd .. && go clean)
	# 编译的第一部分是测试相关的应用的编译
	(cd ../../mrapps && go build $RACE -buildmode=plugin wc.go) || exit 1
	(cd ../../mrapps && go build $RACE -buildmode=plugin indexer.go) || exit 1
	(cd ../../mrapps && go build $RACE -buildmode=plugin mtiming.go) || exit 1
	(cd ../../mrapps && go build $RACE -buildmode=plugin rtiming.go) || exit 1
	(cd ../../mrapps && go build $RACE -buildmode=plugin jobcount.go) || exit 1
	(cd ../../mrapps && go build $RACE -buildmode=plugin early_exit.go) || exit 1
	(cd ../../mrapps && go build $RACE -buildmode=plugin crash.go) || exit 1
	(cd ../../mrapps && go build $RACE -buildmode=plugin nocrash.go) || exit 1
	(cd .. && go build $RACE mrcoordinator.go) || exit 1
	(cd .. && go build $RACE mrworker.go) || exit 1
	(cd .. && go build $RACE mrsequential.go) || exit 1

clean:
	rm -rf mr-tmp mrcoordinator mrsequential mrworker mr-out*