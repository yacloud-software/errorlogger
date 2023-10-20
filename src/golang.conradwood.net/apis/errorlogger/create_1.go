// client create: ErrorLoggerClient
/*
  Created by /home/cnw/devel/go/yatools/src/golang.yacloud.eu/yatools/protoc-gen-cnw/protoc-gen-cnw.go
*/

/* geninfo:
   filename  : protos/golang.conradwood.net/apis/errorlogger/errorlogger.proto
   gopackage : golang.conradwood.net/apis/errorlogger
   importname: ai_0
   clientfunc: GetErrorLogger
   serverfunc: NewErrorLogger
   lookupfunc: ErrorLoggerLookupID
   varname   : client_ErrorLoggerClient_0
   clientname: ErrorLoggerClient
   servername: ErrorLoggerServer
   gsvcname  : errorlogger.ErrorLogger
   lockname  : lock_ErrorLoggerClient_0
   activename: active_ErrorLoggerClient_0
*/

package errorlogger

import (
   "sync"
   "golang.conradwood.net/go-easyops/client"
)
var (
  lock_ErrorLoggerClient_0 sync.Mutex
  client_ErrorLoggerClient_0 ErrorLoggerClient
)

func GetErrorLoggerClient() ErrorLoggerClient { 
    if client_ErrorLoggerClient_0 != nil {
        return client_ErrorLoggerClient_0
    }

    lock_ErrorLoggerClient_0.Lock() 
    if client_ErrorLoggerClient_0 != nil {
       lock_ErrorLoggerClient_0.Unlock()
       return client_ErrorLoggerClient_0
    }

    client_ErrorLoggerClient_0 = NewErrorLoggerClient(client.Connect(ErrorLoggerLookupID()))
    lock_ErrorLoggerClient_0.Unlock()
    return client_ErrorLoggerClient_0
}

func ErrorLoggerLookupID() string { return "errorlogger.ErrorLogger" } // returns the ID suitable for lookup in the registry. treat as opaque, subject to change.

func init() {
   client.RegisterDependency("errorlogger.ErrorLogger")
   AddService("errorlogger.ErrorLogger")
}
