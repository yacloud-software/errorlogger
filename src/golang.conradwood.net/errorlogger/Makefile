.PHONY:	client server
all:	client server
client:
	cd client && go install ${LDFLAGS} errorlogger-client.go
server:
	cd server && go install ${LDFLAGS} errorlogger-server.go `ls -1 *.go|grep -v errorlogger-server.go`
