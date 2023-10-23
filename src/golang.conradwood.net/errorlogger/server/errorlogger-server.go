package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	apb "golang.conradwood.net/apis/auth"
	"golang.conradwood.net/apis/common"
	pb "golang.conradwood.net/apis/errorlogger"
	"golang.conradwood.net/errorlogger/broadcaster"
	"golang.conradwood.net/errorlogger/filelogger"
	"golang.conradwood.net/errorlogger/streamblock"
	"golang.conradwood.net/go-easyops/auth"
	"golang.conradwood.net/go-easyops/authremote"
	"golang.conradwood.net/go-easyops/prometheus"
	"golang.conradwood.net/go-easyops/server"
	"golang.conradwood.net/go-easyops/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"io"
	"os"
	"strings"
	"sync"
)

var (
	debug        = flag.Bool("debug", false, "debug mode")
	errorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "errorlogger_errors_received",
			Help: "V=1 UNIT=none DESC=logs errors received",
		},
		[]string{"grpccode", "service", "method"},
	)

	userlock       sync.Mutex
	port           = flag.Int("port", 4100, "The grpc server port")
	logdir         = flag.String("logdir", "/var/log/errorlogger", "`directory` of errors log")
	logger         *filelogger.FileLogger
	smallLogger    *filelogger.FileLogger
	userlog        *filelogger.FileLogger
	protolog       io.Writer
	peruserlog     = make(map[string]*filelogger.FileLogger)
	logBroadcaster = &broadcaster.Broadcaster{}
)

type echoServer struct {
}

func main() {
	flag.Parse()
	fmt.Printf("Starting ErrorLoggerServer...\n")
	prometheus.MustRegister(errorCounter)
	var err error
	logger, err = filelogger.Open(fmt.Sprintf("%s/all.log", *logdir))
	utils.Bail("failed to open logfile", err)
	smallLogger, err = filelogger.Open(fmt.Sprintf("%s/small.log", *logdir))
	utils.Bail("failed to open logfile", err)
	userlog, err = filelogger.Open(fmt.Sprintf("%s/users.log", *logdir))
	utils.Bail("failed to open userlogfile", err)
	fl, err := filelogger.Open(fmt.Sprintf("%s/proto.log", *logdir))
	utils.Bail("failed to open protologfile", err)
	protolog = streamblock.NewBlockWriter(fl)

	sd := server.NewServerDef()
	sd.NoAuth = true
	sd.SetPort(*port)
	sd.Register = server.Register(
		func(server *grpc.Server) error {
			e := new(echoServer)
			pb.RegisterErrorLoggerServer(server, e)
			return nil
		},
	)
	err = server.ServerStartup(sd)
	utils.Bail("Unable to start server", err)
	os.Exit(0)
}

/************************************
* grpc functions
************************************/

func (e *echoServer) Log(ctx context.Context, req *pb.ErrorLogRequest) (*common.Void, error) {
	var err error
	ctx = authremote.Context()
	email := ""
	var user *apb.User
	if req.UserID != "" {
		user, err = authremote.GetUserByID(ctx, req.UserID)
		if err == nil {
			email = user.Email
		} else {
			fmt.Printf("Unable to get user: %s\n", err)
		}
	}
	if *debug {
		fmt.Printf("Service \"%s\", Method \"%s\", code %d\n", req.ServiceName, req.MethodName, req.ErrorCode)
	}
	l := prometheus.Labels{"grpccode": fmt.Sprintf("%d", req.ErrorCode), "service": req.ServiceName, "method": req.MethodName}
	errorCounter.With(l).Inc()
	storeprotolog(ctx, req)

	svcinfo := "unavailable"
	if req.CallingService != nil {
		svcinfo = fmt.Sprintf("%s(%s)", req.CallingService.ID, req.CallingService.Email)
	}
	var buf bytes.Buffer
	s := fmt.Sprintf("%s: #%05s(%s) [%s->%s.%s] %s %s\n",
		utils.TimestampString(req.Timestamp),
		req.UserID, email,
		svcinfo,
		req.ServiceName, req.MethodName,
		(codes.Code(req.ErrorCode)).String(),
		req.LogMessage,
	)
	buf.WriteString(s)
	for _, m := range req.Messages {
		buf.WriteString(fmt.Sprintf("   %s\n", m.Message))
	}
	logger.WriteString(buf.String())
	if req.UserID != "" {
		userlog.WriteString(buf.String())
	}
	if user != nil {
		fl := getUserLog(user)
		if fl != nil {
			fl.WriteString(buf.String())
		}
	}
	ec := codes.Code(req.ErrorCode)
	if ec != codes.NotFound && ec != codes.PermissionDenied && ec != codes.Unauthenticated {
		smallLogger.WriteString(buf.String())
	}
	return &common.Void{}, nil
}

func getUserLog(u *apb.User) *filelogger.FileLogger {
	if u == nil {
		return nil
	}
	fl := peruserlog[u.ID]
	if fl != nil {
		return fl
	}
	userlock.Lock()
	defer userlock.Unlock()
	fl = peruserlog[u.ID]
	if fl != nil {
		return fl
	}
	ua := u.Abbrev
	if ua == "" {
		ua = u.ID
	}
	s := fmt.Sprintf("%s/%s.log", *logdir, ua)
	fl, err := filelogger.Open(s)
	utils.Bail("failed to open logfile", err)
	peruserlog[u.ID] = fl
	return fl
}
func storeprotolog(ctx context.Context, req *pb.ErrorLogRequest) {
	pl := &pb.ProtoLog{
		Err:     req,
		User:    auth.GetUser(ctx),
		Service: auth.GetService(ctx),
	}
	bs, err := utils.MarshalBytes(pl)
	if err != nil {
		fmt.Printf("Failed to marshal error proto: %s\n", err)
		return
	}
	_, err = protolog.Write(bs)
	if err != nil {
		fmt.Printf("failed to write proto: %s\n", err)
		return
	}
	logBroadcaster.NewData(pl)

}
func (e *echoServer) ReadLog(req *pb.ReadLogRequest, srv pb.ErrorLogger_ReadLogServer) error {
	fmt.Printf("Listener added for services \"%s\"\n", strings.Join(req.Services, " "))
	file, err := os.Open(fmt.Sprintf("%s/proto.log", *logdir))
	if err != nil {
		return err
	}
	m := &proto_matcher{req: req}
	br := streamblock.NewSeekableBlockReader(file)
	max_to_read := 100
	// send from log
	block_counter := 0
	matching_block_counter := 0
	var bys []byte
	for {
		if block_counter == 0 {
			bys, err = br.ReadLastBlock()
		} else {
			bys, err = br.ReadPreviousBlock()
		}
		block_counter++
		if err != nil {
			break
		}
		if !m.Match(bys) {
			continue
		}
		matching_block_counter++
		if matching_block_counter >= max_to_read {
			break
		}
		pl := m.lastProto()

		err = srv.Send(pl)
		if err != nil {
			return err
		}
	}
	fmt.Printf("BlockCounter: %d, MatchingBlockCounter: %d\n", block_counter, matching_block_counter)
	// send live
	err = logBroadcaster.Handle(srv, func(srv any, data any) error {
		d := data.(*pb.ProtoLog)
		if !match_proto(req, d) {
			return nil
		}
		return srv.(pb.ErrorLogger_ReadLogServer).Send(d)
	})
	if err != nil {
		return err
	}
	return nil
}

type proto_matcher struct {
	req *pb.ReadLogRequest
	pl  *pb.ProtoLog
}

func (p *proto_matcher) Match(b []byte) bool {
	pl := &pb.ProtoLog{}
	err := utils.UnmarshalBytes(b, pl)
	if err != nil {
		return false
	}
	p.pl = pl
	return match_proto(p.req, p.pl)
}
func (p *proto_matcher) lastProto() *pb.ProtoLog {
	return p.pl
}
func match_proto(req *pb.ReadLogRequest, pl *pb.ProtoLog) bool {
	if req == nil {
		return true
	}
	if len(req.Services) == 0 {
		return true
	}
	if pl.Err == nil {
		return false
	}
	svc := strings.ToLower(pl.Err.ServiceName)
	for _, s := range req.Services {
		sl := strings.ToLower(s)
		if strings.Contains(svc, sl) {
			return true
		}
	}
	return false
}
