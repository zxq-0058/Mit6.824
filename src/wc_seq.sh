cd main
go build -race -buildmode=plugin -gcflags "all=-N -l"  ../mrapps/wc.go
rm rm-out*
go build -race -gcflags "all=-N -l" mrsequential.go wc.so