// Code generated by protoc-gen-go.
// source: grpc.proto
// DO NOT EDIT!

/*
Package plumbing is a generated protocol buffer package.

It is generated from these files:
	grpc.proto

It has these top-level messages:
	EnvelopeData
	PushResponse
	SubscriptionRequest
	Filter
	LogFilter
	MetricFilter
	Response
	BatchResponse
*/
package plumbing

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type EnvelopeData struct {
	Payload []byte `protobuf:"bytes,1,opt,name=payload,proto3" json:"payload,omitempty"`
}

func (m *EnvelopeData) Reset()                    { *m = EnvelopeData{} }
func (m *EnvelopeData) String() string            { return proto.CompactTextString(m) }
func (*EnvelopeData) ProtoMessage()               {}
func (*EnvelopeData) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *EnvelopeData) GetPayload() []byte {
	if m != nil {
		return m.Payload
	}
	return nil
}

type PushResponse struct {
}

func (m *PushResponse) Reset()                    { *m = PushResponse{} }
func (m *PushResponse) String() string            { return proto.CompactTextString(m) }
func (*PushResponse) ProtoMessage()               {}
func (*PushResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

type SubscriptionRequest struct {
	ShardID string  `protobuf:"bytes,1,opt,name=shardID" json:"shardID,omitempty"`
	Filter  *Filter `protobuf:"bytes,2,opt,name=filter" json:"filter,omitempty"`
}

func (m *SubscriptionRequest) Reset()                    { *m = SubscriptionRequest{} }
func (m *SubscriptionRequest) String() string            { return proto.CompactTextString(m) }
func (*SubscriptionRequest) ProtoMessage()               {}
func (*SubscriptionRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *SubscriptionRequest) GetShardID() string {
	if m != nil {
		return m.ShardID
	}
	return ""
}

func (m *SubscriptionRequest) GetFilter() *Filter {
	if m != nil {
		return m.Filter
	}
	return nil
}

type Filter struct {
	AppID string `protobuf:"bytes,1,opt,name=appID" json:"appID,omitempty"`
	// Types that are valid to be assigned to Message:
	//	*Filter_Log
	//	*Filter_Metric
	Message isFilter_Message `protobuf_oneof:"Message"`
}

func (m *Filter) Reset()                    { *m = Filter{} }
func (m *Filter) String() string            { return proto.CompactTextString(m) }
func (*Filter) ProtoMessage()               {}
func (*Filter) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

type isFilter_Message interface {
	isFilter_Message()
}

type Filter_Log struct {
	Log *LogFilter `protobuf:"bytes,2,opt,name=log,oneof"`
}
type Filter_Metric struct {
	Metric *MetricFilter `protobuf:"bytes,3,opt,name=metric,oneof"`
}

func (*Filter_Log) isFilter_Message()    {}
func (*Filter_Metric) isFilter_Message() {}

func (m *Filter) GetMessage() isFilter_Message {
	if m != nil {
		return m.Message
	}
	return nil
}

func (m *Filter) GetAppID() string {
	if m != nil {
		return m.AppID
	}
	return ""
}

func (m *Filter) GetLog() *LogFilter {
	if x, ok := m.GetMessage().(*Filter_Log); ok {
		return x.Log
	}
	return nil
}

func (m *Filter) GetMetric() *MetricFilter {
	if x, ok := m.GetMessage().(*Filter_Metric); ok {
		return x.Metric
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*Filter) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _Filter_OneofMarshaler, _Filter_OneofUnmarshaler, _Filter_OneofSizer, []interface{}{
		(*Filter_Log)(nil),
		(*Filter_Metric)(nil),
	}
}

func _Filter_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*Filter)
	// Message
	switch x := m.Message.(type) {
	case *Filter_Log:
		b.EncodeVarint(2<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Log); err != nil {
			return err
		}
	case *Filter_Metric:
		b.EncodeVarint(3<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Metric); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("Filter.Message has unexpected type %T", x)
	}
	return nil
}

func _Filter_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*Filter)
	switch tag {
	case 2: // Message.log
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(LogFilter)
		err := b.DecodeMessage(msg)
		m.Message = &Filter_Log{msg}
		return true, err
	case 3: // Message.metric
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(MetricFilter)
		err := b.DecodeMessage(msg)
		m.Message = &Filter_Metric{msg}
		return true, err
	default:
		return false, nil
	}
}

func _Filter_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*Filter)
	// Message
	switch x := m.Message.(type) {
	case *Filter_Log:
		s := proto.Size(x.Log)
		n += proto.SizeVarint(2<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case *Filter_Metric:
		s := proto.Size(x.Metric)
		n += proto.SizeVarint(3<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

type LogFilter struct {
}

func (m *LogFilter) Reset()                    { *m = LogFilter{} }
func (m *LogFilter) String() string            { return proto.CompactTextString(m) }
func (*LogFilter) ProtoMessage()               {}
func (*LogFilter) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

type MetricFilter struct {
}

func (m *MetricFilter) Reset()                    { *m = MetricFilter{} }
func (m *MetricFilter) String() string            { return proto.CompactTextString(m) }
func (*MetricFilter) ProtoMessage()               {}
func (*MetricFilter) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

// Note: Ideally this would be EnvelopeData but for the time being we do not
// want to pay the cost of planning an upgrade path for this to be renamed.
type Response struct {
	Payload []byte `protobuf:"bytes,1,opt,name=payload,proto3" json:"payload,omitempty"`
}

func (m *Response) Reset()                    { *m = Response{} }
func (m *Response) String() string            { return proto.CompactTextString(m) }
func (*Response) ProtoMessage()               {}
func (*Response) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{6} }

func (m *Response) GetPayload() []byte {
	if m != nil {
		return m.Payload
	}
	return nil
}

type BatchResponse struct {
	Payload [][]byte `protobuf:"bytes,1,rep,name=payload,proto3" json:"payload,omitempty"`
}

func (m *BatchResponse) Reset()                    { *m = BatchResponse{} }
func (m *BatchResponse) String() string            { return proto.CompactTextString(m) }
func (*BatchResponse) ProtoMessage()               {}
func (*BatchResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{7} }

func (m *BatchResponse) GetPayload() [][]byte {
	if m != nil {
		return m.Payload
	}
	return nil
}

func init() {
	proto.RegisterType((*EnvelopeData)(nil), "plumbing.EnvelopeData")
	proto.RegisterType((*PushResponse)(nil), "plumbing.PushResponse")
	proto.RegisterType((*SubscriptionRequest)(nil), "plumbing.SubscriptionRequest")
	proto.RegisterType((*Filter)(nil), "plumbing.Filter")
	proto.RegisterType((*LogFilter)(nil), "plumbing.LogFilter")
	proto.RegisterType((*MetricFilter)(nil), "plumbing.MetricFilter")
	proto.RegisterType((*Response)(nil), "plumbing.Response")
	proto.RegisterType((*BatchResponse)(nil), "plumbing.BatchResponse")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for Doppler service

type DopplerClient interface {
	Subscribe(ctx context.Context, in *SubscriptionRequest, opts ...grpc.CallOption) (Doppler_SubscribeClient, error)
	BatchSubscribe(ctx context.Context, in *SubscriptionRequest, opts ...grpc.CallOption) (Doppler_BatchSubscribeClient, error)
}

type dopplerClient struct {
	cc *grpc.ClientConn
}

func NewDopplerClient(cc *grpc.ClientConn) DopplerClient {
	return &dopplerClient{cc}
}

func (c *dopplerClient) Subscribe(ctx context.Context, in *SubscriptionRequest, opts ...grpc.CallOption) (Doppler_SubscribeClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_Doppler_serviceDesc.Streams[0], c.cc, "/plumbing.Doppler/Subscribe", opts...)
	if err != nil {
		return nil, err
	}
	x := &dopplerSubscribeClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Doppler_SubscribeClient interface {
	Recv() (*Response, error)
	grpc.ClientStream
}

type dopplerSubscribeClient struct {
	grpc.ClientStream
}

func (x *dopplerSubscribeClient) Recv() (*Response, error) {
	m := new(Response)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *dopplerClient) BatchSubscribe(ctx context.Context, in *SubscriptionRequest, opts ...grpc.CallOption) (Doppler_BatchSubscribeClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_Doppler_serviceDesc.Streams[1], c.cc, "/plumbing.Doppler/BatchSubscribe", opts...)
	if err != nil {
		return nil, err
	}
	x := &dopplerBatchSubscribeClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Doppler_BatchSubscribeClient interface {
	Recv() (*BatchResponse, error)
	grpc.ClientStream
}

type dopplerBatchSubscribeClient struct {
	grpc.ClientStream
}

func (x *dopplerBatchSubscribeClient) Recv() (*BatchResponse, error) {
	m := new(BatchResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Server API for Doppler service

type DopplerServer interface {
	Subscribe(*SubscriptionRequest, Doppler_SubscribeServer) error
	BatchSubscribe(*SubscriptionRequest, Doppler_BatchSubscribeServer) error
}

func RegisterDopplerServer(s *grpc.Server, srv DopplerServer) {
	s.RegisterService(&_Doppler_serviceDesc, srv)
}

func _Doppler_Subscribe_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SubscriptionRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(DopplerServer).Subscribe(m, &dopplerSubscribeServer{stream})
}

type Doppler_SubscribeServer interface {
	Send(*Response) error
	grpc.ServerStream
}

type dopplerSubscribeServer struct {
	grpc.ServerStream
}

func (x *dopplerSubscribeServer) Send(m *Response) error {
	return x.ServerStream.SendMsg(m)
}

func _Doppler_BatchSubscribe_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SubscriptionRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(DopplerServer).BatchSubscribe(m, &dopplerBatchSubscribeServer{stream})
}

type Doppler_BatchSubscribeServer interface {
	Send(*BatchResponse) error
	grpc.ServerStream
}

type dopplerBatchSubscribeServer struct {
	grpc.ServerStream
}

func (x *dopplerBatchSubscribeServer) Send(m *BatchResponse) error {
	return x.ServerStream.SendMsg(m)
}

var _Doppler_serviceDesc = grpc.ServiceDesc{
	ServiceName: "plumbing.Doppler",
	HandlerType: (*DopplerServer)(nil),
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Subscribe",
			Handler:       _Doppler_Subscribe_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "BatchSubscribe",
			Handler:       _Doppler_BatchSubscribe_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "grpc.proto",
}

// Client API for DopplerIngestor service

type DopplerIngestorClient interface {
	Pusher(ctx context.Context, opts ...grpc.CallOption) (DopplerIngestor_PusherClient, error)
}

type dopplerIngestorClient struct {
	cc *grpc.ClientConn
}

func NewDopplerIngestorClient(cc *grpc.ClientConn) DopplerIngestorClient {
	return &dopplerIngestorClient{cc}
}

func (c *dopplerIngestorClient) Pusher(ctx context.Context, opts ...grpc.CallOption) (DopplerIngestor_PusherClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_DopplerIngestor_serviceDesc.Streams[0], c.cc, "/plumbing.DopplerIngestor/Pusher", opts...)
	if err != nil {
		return nil, err
	}
	x := &dopplerIngestorPusherClient{stream}
	return x, nil
}

type DopplerIngestor_PusherClient interface {
	Send(*EnvelopeData) error
	CloseAndRecv() (*PushResponse, error)
	grpc.ClientStream
}

type dopplerIngestorPusherClient struct {
	grpc.ClientStream
}

func (x *dopplerIngestorPusherClient) Send(m *EnvelopeData) error {
	return x.ClientStream.SendMsg(m)
}

func (x *dopplerIngestorPusherClient) CloseAndRecv() (*PushResponse, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(PushResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Server API for DopplerIngestor service

type DopplerIngestorServer interface {
	Pusher(DopplerIngestor_PusherServer) error
}

func RegisterDopplerIngestorServer(s *grpc.Server, srv DopplerIngestorServer) {
	s.RegisterService(&_DopplerIngestor_serviceDesc, srv)
}

func _DopplerIngestor_Pusher_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(DopplerIngestorServer).Pusher(&dopplerIngestorPusherServer{stream})
}

type DopplerIngestor_PusherServer interface {
	SendAndClose(*PushResponse) error
	Recv() (*EnvelopeData, error)
	grpc.ServerStream
}

type dopplerIngestorPusherServer struct {
	grpc.ServerStream
}

func (x *dopplerIngestorPusherServer) SendAndClose(m *PushResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *dopplerIngestorPusherServer) Recv() (*EnvelopeData, error) {
	m := new(EnvelopeData)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _DopplerIngestor_serviceDesc = grpc.ServiceDesc{
	ServiceName: "plumbing.DopplerIngestor",
	HandlerType: (*DopplerIngestorServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Pusher",
			Handler:       _DopplerIngestor_Pusher_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "grpc.proto",
}

func init() { proto.RegisterFile("grpc.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 427 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x93, 0xcf, 0x6e, 0xd3, 0x40,
	0x10, 0xc6, 0xe3, 0x46, 0x38, 0xcd, 0x34, 0x94, 0x32, 0x45, 0xd4, 0x0a, 0x20, 0x95, 0x15, 0x12,
	0xee, 0xc5, 0x54, 0x81, 0x23, 0xa7, 0x10, 0x10, 0x91, 0x52, 0x81, 0xcc, 0x09, 0x71, 0x5a, 0xbb,
	0x83, 0x63, 0xc9, 0xdd, 0x5d, 0x76, 0xd7, 0x48, 0xdc, 0x79, 0x33, 0x5e, 0x0c, 0xf9, 0xbf, 0x5b,
	0x4c, 0xd3, 0xe3, 0xcc, 0x7c, 0xf3, 0xcd, 0xee, 0x4f, 0x33, 0x00, 0x89, 0x56, 0x71, 0xa0, 0xb4,
	0xb4, 0x12, 0xf7, 0x55, 0x96, 0x5f, 0x45, 0xa9, 0x48, 0x98, 0x0f, 0xb3, 0xf7, 0xe2, 0x27, 0x65,
	0x52, 0xd1, 0x8a, 0x5b, 0x8e, 0x1e, 0x4c, 0x14, 0xff, 0x95, 0x49, 0x7e, 0xe9, 0x39, 0xa7, 0x8e,
	0x3f, 0x0b, 0x9b, 0x90, 0x1d, 0xc2, 0xec, 0x73, 0x6e, 0xb6, 0x21, 0x19, 0x25, 0x85, 0x21, 0xf6,
	0x15, 0x8e, 0xbf, 0xe4, 0x91, 0x89, 0x75, 0xaa, 0x6c, 0x2a, 0x45, 0x48, 0x3f, 0x72, 0x32, 0xb6,
	0x30, 0x30, 0x5b, 0xae, 0x2f, 0xd7, 0xab, 0xd2, 0x60, 0x1a, 0x36, 0x21, 0xfa, 0xe0, 0x7e, 0x4f,
	0x33, 0x4b, 0xda, 0xdb, 0x3b, 0x75, 0xfc, 0x83, 0xc5, 0x51, 0xd0, 0xbc, 0x22, 0xf8, 0x50, 0xe6,
	0xc3, 0xba, 0xce, 0x7e, 0x3b, 0xe0, 0x56, 0x29, 0x7c, 0x04, 0xf7, 0xb8, 0x52, 0xad, 0x59, 0x15,
	0xe0, 0x4b, 0x18, 0x67, 0x32, 0xa9, 0x7d, 0x8e, 0x3b, 0x9f, 0x8d, 0x4c, 0xaa, 0xbe, 0x8f, 0xa3,
	0xb0, 0x50, 0xe0, 0x39, 0xb8, 0x57, 0x64, 0x75, 0x1a, 0x7b, 0xe3, 0x52, 0xfb, 0xb8, 0xd3, 0x5e,
	0x94, 0xf9, 0x56, 0x5e, 0xeb, 0x96, 0x53, 0x98, 0x5c, 0x90, 0x31, 0x3c, 0x21, 0x76, 0x00, 0xd3,
	0xd6, 0xb0, 0xf8, 0x7e, 0xbf, 0x83, 0xbd, 0x80, 0xfd, 0x06, 0xc5, 0x2d, 0xd0, 0xce, 0xe0, 0xfe,
	0x92, 0xdb, 0x78, 0x3b, 0x2c, 0x1d, 0xf7, 0xa5, 0xaf, 0xe0, 0xe4, 0x9d, 0x14, 0x96, 0xa7, 0x82,
	0x74, 0x35, 0xc9, 0x34, 0x4c, 0x07, 0x21, 0xb0, 0x37, 0xe0, 0xfd, 0xdb, 0xb0, 0x73, 0xcc, 0x19,
	0x3c, 0x0c, 0x29, 0x26, 0x61, 0x37, 0x32, 0xd9, 0x31, 0x20, 0x00, 0xec, 0x4b, 0x77, 0x59, 0x2f,
	0xfe, 0xec, 0xc1, 0x64, 0x25, 0x95, 0xca, 0x48, 0xe3, 0x12, 0xa6, 0xf5, 0x76, 0x44, 0x84, 0xcf,
	0x3a, 0xea, 0x03, 0x2b, 0x33, 0xc7, 0xae, 0xdc, 0x6e, 0xd7, 0xe8, 0xdc, 0xc1, 0x0d, 0x1c, 0x96,
	0xf0, 0xee, 0x6c, 0x74, 0xd2, 0x95, 0xaf, 0x51, 0x2f, 0xdd, 0xbe, 0xc1, 0xd1, 0x4d, 0x5c, 0xf8,
	0xbc, 0x6b, 0xf8, 0x0f, 0xfb, 0x39, 0xbb, 0x4d, 0xd2, 0xd8, 0xe3, 0x1a, 0xa0, 0x43, 0x85, 0x4f,
	0xfa, 0x1f, 0xba, 0xc1, 0x7a, 0xfe, 0x74, 0xb8, 0xd8, 0x58, 0x2d, 0x3e, 0xc1, 0x83, 0x1a, 0xe2,
	0x5a, 0x24, 0x64, 0xac, 0xd4, 0xf8, 0x16, 0xdc, 0xe2, 0xf4, 0x48, 0x63, 0x6f, 0x7f, 0xfb, 0x67,
	0x3b, 0xef, 0xe5, 0xaf, 0x1d, 0xe9, 0xc8, 0x77, 0x22, 0xb7, 0xbc, 0xf9, 0xd7, 0x7f, 0x03, 0x00,
	0x00, 0xff, 0xff, 0x92, 0x34, 0x48, 0xe2, 0x01, 0x04, 0x00, 0x00,
}
