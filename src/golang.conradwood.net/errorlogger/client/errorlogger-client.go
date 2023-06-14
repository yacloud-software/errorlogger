package main

import (
	"flag"
	"fmt"
	_ "golang.conradwood.net/apis/common"
	pb "golang.conradwood.net/apis/errorlogger"
	"golang.conradwood.net/go-easyops/auth"
	"golang.conradwood.net/go-easyops/authremote"
	"golang.conradwood.net/go-easyops/utils"
	"os"
	"strings"
	"time"
)

var (
	echoClient pb.ErrorLoggerClient
	sn         = flag.String("service", "", "service name to filter on")
	listen     = flag.Bool("listen", false, "listen for errors in realtime")
)

func main() {
	flag.Parse()
	if *listen {
		utils.Bail("failed to listen", Listen())
		os.Exit(0)
	}
	echoClient = pb.GetErrorLoggerClient()

	// a context with authentication
	ctx := authremote.Context()

	l := &pb.ErrorLogRequest{}
	_, err := echoClient.Log(ctx, l)
	utils.Bail("Failed to ping server", err)

	fmt.Printf("Done.\n")
	os.Exit(0)
}

func getServiceNames() []string {
	if *sn == "" {
		return nil
	}
	svs := strings.Split(*sn, ",")
	for i, s := range svs {
		s = strings.Trim(s, " ")
		svs[i] = s
	}
	return svs
}
func Listen() error {
	ctx := authremote.ContextWithTimeout(time.Duration(60) * time.Minute)
	rlr := &pb.ReadLogRequest{
		Services: getServiceNames(),
	}

	srv, err := pb.GetErrorLoggerClient().ReadLog(ctx, rlr)
	if err != nil {
		return err
	}
	fmt.Printf("Listening for services \"%s\"...\n", strings.Join(rlr.Services, " "))
	for {
		r, err := srv.Recv()
		if err != nil {
			return err
		}
		//fmt.Printf("LOG: %v\n", r)
		e := r.Err
		cus := auth.UserIDString(r.User)
		fmt.Printf("%s %s %s %d %s\n", strlen(cus, 20), strlen(e.UserID, 6), strlen(e.ServiceName+"/"+e.MethodName, 50), e.ErrorCode, e.ErrorMessage)
		for _, m := range e.Messages {
			for _, ct := range m.CallTraces {
				fmt.Printf("      -> %v\n", ct)
			}
		}
	}
}

func strlen(s string, ln int) string {
	if len(s) > ln {
		return s[:ln-3] + "..."
	}
	for len(s) < ln {
		s = s + " "
	}
	return s
}
