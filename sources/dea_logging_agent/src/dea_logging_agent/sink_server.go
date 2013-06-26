package dea_logging_agent

type SinkServer interface {
	Send([]byte)
}

type TcpSinkServer struct {}

func (sinkServer *TcpSinkServer) Send(data []byte ) {

}
